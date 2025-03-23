package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/fatih/color"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/sibiraj-s/depot-go-ecr-example/aws"
	"golang.org/x/sync/errgroup"
)

type BuildOptions struct {
	Registry       *aws.ECRRegistry
	Region         string
	Tag            string
	DockerfilePath string
	Arch           string
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

var dockerFileName = "Dockerfile"

func newLine() {
	fmt.Println("")
}

func main() {
	// load depot variables from env
	depotToken := os.Getenv("DEPOT_TOKEN")
	depotProjectId := os.Getenv("DEPOT_PROJECT_ID")

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

	// 2. Check if dockerfile exists
	dockerFilePath := JoinPath(cloneDir, dockerFileName)
	if !FileExists(dockerFilePath) {
		fmt.Println("Dockerfile not found in", cloneDir)
		return
	}

	// You can use a context with timeout to cancel the build if you would like.
	ctx := context.Background()

	// this will be dynamically generated for each job/deployment.
	opts := BuildOptions{
		DockerfilePath: dockerFilePath,
		Registry:       registry,
		Region:         inputs.Region,
		Tag:            aws.ImageTag(registry, workflowId),
		Arch:           inputs.Arch,
	}

	// 3. Register a new build.
	req := &cliv1.CreateBuildRequest{
		ProjectId: depotProjectId,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{opts.Tag},
			},
		},
	}
	build, err := build.NewBuild(ctx, req, depotToken)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Waiting for an instance to pickup the build...")

	// Set the buildErr to any error that represents the build failing.
	var buildErr error
	defer build.Finish(buildErr)

	// 4. Acquire a buildkit machine.
	var buildkit *machine.Machine
	buildkit, buildErr = machine.Acquire(ctx, build.ID, build.Token, inputs.Arch)
	if buildErr != nil {
		return
	}
	defer buildkit.Release()

	// 5. Check buildkitd readiness. When the buildkitd starts, it may take
	// quite a while to be ready to accept connections when it loads a large boltdb.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelConnect()

	var buildkitClient *client.Client
	buildkitClient, buildErr = buildkit.Connect(connectCtx)
	if buildErr != nil {
		return
	}

	// 6. Use the buildkit client to build the image.
	buildErr = buildImage(ctx, buildkitClient, opts)
	if buildErr != nil {
		return
	}
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
	repoDir := filepath.Dir(opts.DockerfilePath)

	eg.Go(func() error {
		opts := client.SolveOpt{
			Frontend: "dockerfile.v0",
			FrontendAttrs: map[string]string{
				"filename": dockerFileName,
				"platform": fmt.Sprintf("linux/%s", opts.Arch),
			},
			LocalDirs: map[string]string{
				"dockerfile": repoDir,
				"context":    repoDir,
			},
			Exports:  []client.ExportEntry{exportEntry},
			Session:  []session.Attachable{NewBuildkitAuthProvider(ecrCreds, opts.Registry)},
			Internal: true, // Prevent recording the build steps and traces in buildkit as it is _very_ slow.
		}

		res, err = buildkitClient.Solve(ctx, nil, opts, ch)
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
	RemoveDir(repoDir)
	fmt.Println("Clone directory removed.")

	newLine()
	c := color.New(color.FgGreen)
	c.Printf("Image build complete %s\n", id)
	return nil
}
