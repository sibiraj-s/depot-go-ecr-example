package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/fatih/color"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/connhelper"
	_ "github.com/moby/buildkit/client/connhelper/ssh" // Register SSH connection helper
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/sibiraj-s/depot-go-ecr-example/aws"
	"golang.org/x/sync/errgroup"
)

type Builder string

const (
	BUILDER_DOCKER   Builder = "docker"
	BUILDER_RAILPACK Builder = "railpack"
)

type BuildOptions struct {
	Registry       *aws.ECRRegistry
	Region         string
	Tag            string
	DockerfilePath string
	Arch           string
	RepoDirPath    string
	Builder        Builder
}

type Annotations struct {
	RawManifest string `json:"depot.containerimage.manifest,omitempty"`
}

type Descriptor struct {
	MediaType   string      `json:"mediaType,omitempty"`
	Digest      string      `json:"digest,omitempty"`
	Size        int64       `json:"size,omitempty"`
	Annotations Annotations `json:"annotations,omitempty"`
}

func newLine() {
	fmt.Println("")
}

var RAILPACK_VERSION = "v0.17.1"
var railpackFrontend = fmt.Sprintf("ghcr.io/railwayapp/railpack-frontend:%s", RAILPACK_VERSION)

func main() {
	// get the repo to clone by prompting the user
	inputs := AskInputs()

	// print the inputs
	fmt.Println("\nRepository URL: ", inputs.Repo)
	fmt.Println("ECR Registry: ", inputs.RegistryURL)
	fmt.Println("Region: ", inputs.Region)
	fmt.Println("Architecture: ", inputs.Arch)
	newLine()

	registry, err := aws.ExtractEcrRegisty(inputs.RegistryURL)
	if err != nil {
		log.Fatal(err)
	}

	workflowId := GenerateUniqueId()
	fmt.Println("Starting workflow: ", workflowId)
	newLine()

	// 1. Clone the repo
	fmt.Println("Cloning repository...")
	cloneDir := CloneRepo(inputs.Repo, workflowId)
	fmt.Println("Repository successfully cloned to: ", cloneDir)
	newLine()

	builder := BUILDER_DOCKER

	// 2. Check if dockerfile exists
	if !FileExists(JoinPath(cloneDir, inputs.DockerfilePath)) {
		// if dockerfile does not exist, use railpack frontend
		fmt.Printf("Dockerfile not found in %s. Falling back to Railpack %s...\n", cloneDir, RAILPACK_VERSION)
		builder = BUILDER_RAILPACK
	}

	// prepare railpack
	if builder == BUILDER_RAILPACK {
		// check if railpack is installed
		if _, err := exec.LookPath("railpack"); err != nil {
			log.Fatalf("railpack is not installed: %v", err)
		}

		fmt.Println("Preparing Railpack plan...")
		railpackPlanJsonPath := filepath.Join(cloneDir, "railpack-plan.json")
		railpackInfoJsonPath := filepath.Join(cloneDir, "railpack-info.json")

		args := []string{
			"prepare",
			cloneDir,
			"--plan-out", railpackPlanJsonPath,
			"--info-out", railpackInfoJsonPath,
		}

		// execute the railpack prepare command
		railpackCmd := exec.Command("railpack", args...)
		railpackCmd.Stdout = os.Stdout
		railpackCmd.Stderr = os.Stderr
		err = railpackCmd.Run()
		if err != nil {
			log.Fatalf("failed to prepare Railpack plan: %v", err)
		}

		if _, err = os.Stat(railpackPlanJsonPath); err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("railpack-plan.json was not created: %v", err)
			}
			log.Fatalf("failed to check for railpack-plan.json: %v", err)
		}
	}

	// You can use a context with timeout to cancel the build if you would like.
	ctx := context.Background()

	// this will be dynamically generated for each job/deployment.
	opts := BuildOptions{
		RepoDirPath:    cloneDir,
		DockerfilePath: inputs.DockerfilePath,
		Registry:       registry,
		Region:         inputs.Region,
		Tag:            aws.ImageTag(registry, workflowId),
		Arch:           inputs.Arch,
		Builder:        builder,
	}

	buildkitClient, cleanup, err := getBuildkitClient(ctx, inputs, opts)
	if err != nil {
		log.Fatal(err)
	}

	// Use the buildkit client to build the image.
	buildErr := buildImage(ctx, buildkitClient, opts)
	cleanup(buildErr)
	if buildErr != nil {
		log.Fatal(buildErr)
	}
}

func getBuildkitClient(ctx context.Context, inputs PromptResults, opts BuildOptions) (*client.Client, func(error), error) {
	if inputs.RemoteBuilderAddress != "" {
		fmt.Printf("Connecting to custom builder at %s...\n", inputs.RemoteBuilderAddress)

		var buildkitClient *client.Client
		var err error

		// Check if the address requires a connection helper (e.g., ssh://)
		helper, err := connhelper.GetConnectionHelper(inputs.RemoteBuilderAddress)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get connection helper: %w", err)
		}

		var dialer client.ClientOpt
		address := inputs.RemoteBuilderAddress
		if helper != nil {
			address = ""
			dialer = client.WithContextDialer(helper.ContextDialer)
		}

		buildkitClient, err = client.New(ctx, address, dialer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to remote builder: %w", err)
		}

		cleanup := func(_ error) {
			buildkitClient.Close()
		}
		return buildkitClient, cleanup, nil
	}

	// load depot variables from env
	depotToken := os.Getenv("DEPOT_TOKEN")
	depotProjectId := os.Getenv("DEPOT_PROJECT_ID")

	// Register a new build with Depot.
	req := &cliv1.CreateBuildRequest{
		ProjectId: depotProjectId,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{opts.Tag},
			},
		},
	}
	depotBuild, err := build.NewBuild(ctx, req, depotToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create depot build: %w", err)
	}

	fmt.Println("Waiting for a depot machine to pickup the build...")

	// Acquire a buildkit machine.
	buildkit, err := machine.Acquire(ctx, depotBuild.ID, depotBuild.Token, inputs.Arch)
	if err != nil {
		depotBuild.Finish(err)
		return nil, nil, fmt.Errorf("failed to acquire buildkit machine: %w", err)
	}

	// Check buildkitd readiness. When the buildkitd starts, it may take
	// quite a while to be ready to accept connections when it loads a large boltdb.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelConnect()

	buildkitClient, err := buildkit.Connect(connectCtx)
	if err != nil {
		buildkit.Release()
		depotBuild.Finish(err)
		return nil, nil, fmt.Errorf("failed to connect to buildkit: %w", err)
	}

	cleanup := func(buildErr error) {
		buildkit.Release()
		depotBuild.Finish(buildErr)
	}

	return buildkitClient, cleanup, nil
}

func buildImage(ctx context.Context, buildkitClient *client.Client, opts BuildOptions) error {
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	var res *client.SolveResponse
	var err error

	exportEntry := client.ExportEntry{
		Type: "image",
		Attrs: map[string]string{
			"name":           opts.Tag,
			"oci-mediatypes": "true",
			"push":           "true",
		},
	}

	ecrCreds := aws.GetEcrCreds(ctx, opts.Region, opts.Registry)

	eg.Go(func() error {
		frontendAttrs := map[string]string{
			"filename": opts.DockerfilePath,
			"platform": fmt.Sprintf("linux/%s", opts.Arch),
			// "source":   "docker/dockerfile", // default source
		}

		solveOpts := client.SolveOpt{
			Frontend:      "dockerfile.v0",
			FrontendAttrs: frontendAttrs,
			LocalDirs: map[string]string{
				"dockerfile": opts.RepoDirPath,
				"context":    opts.RepoDirPath,
			},
			Exports:  []client.ExportEntry{exportEntry},
			Session:  []session.Attachable{NewBuildkitAuthProvider(ecrCreds, opts.Registry)},
			Internal: true, // Prevent recording the build steps and traces in buildkit as it is _very_ slow.
		}

		// override default gateway to use custom frontend
		// Ref: https://docs.docker.com/build/buildkit/frontend/#dockerfile-frontend
		// https://github.com/docker/buildx/blob/abf6ab4a377ff196714ba06d8e62407ef1750549/build/opt.go#L132-L137
		if opts.Builder == BUILDER_RAILPACK {
			solveOpts.Frontend = "gateway.v0"
			solveOpts.FrontendAttrs["source"] = railpackFrontend
			solveOpts.FrontendAttrs["cmdline"] = railpackFrontend
		}

		res, err = buildkitClient.Solve(ctx, nil, solveOpts, ch)
		return err
	})

	eg.Go(func() error {
		display, err := progressui.NewDisplay(os.Stdout, progressui.AutoMode)
		if err != nil {
			return err
		}

		_, err = display.UpdateFrom(context.TODO(), ch)
		return err
	})

	if err := eg.Wait(); err != nil {
		fmt.Printf("Build error: %v\n", err)
		return err
	}

	id := res.ExporterResponse["containerimage.digest"]
	encoded := res.ExporterResponse["containerimage.descriptor"]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}

	var descriptor Descriptor
	err = json.Unmarshal(decoded, &descriptor)
	if err != nil {
		return err
	}

	fmt.Println("Removing clone directory...")
	RemoveDir(opts.RepoDirPath)
	fmt.Println("Clone directory removed.")

	newLine()
	c := color.New(color.FgGreen)
	c.Printf("Image build complete %s\n", id)
	return nil
}
