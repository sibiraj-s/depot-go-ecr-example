package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
)

func validateRequired(input string) func(string) error {
	return func(value string) error {
		if value == "" {
			return fmt.Errorf("%s is required", input)
		}
		return nil
	}
}

func validateProtocol(input string, protocols []string, optional bool) func(string) error {
	return func(value string) error {
		if value == "" && optional {
			return nil
		}

		// if value is not empty, check if it starts with any of the given protocols
		// regardless of optional
		// supports only the given protocols
		for _, protocol := range protocols {
			if strings.HasPrefix(value, protocol) {
				return nil
			}
		}
		return fmt.Errorf("unsupported %s: %s. Only %s are supported", input, value, strings.Join(protocols, ", "))
	}
}

type PromptResults struct {
	Repo                 string
	RegistryURL          string
	Region               string
	Arch                 string
	DockerfilePath       string
	RemoteBuilderAddress string
}

func detectArch() string {
	arch := runtime.GOARCH

	if arch == "arm64" || arch == "arm" {
		return "arm64"
	}

	return "amd64"
}

func AskInputs() PromptResults {
	var (
		repo              string
		registry          string
		region            string
		arch              string = detectArch()
		dockerfilePath    string = "Dockerfile"
		remoteBuilderAddr string
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter the repository to clone").
				Value(&repo).
				Placeholder("https://github.com/username/repo").
				Validate(validateRequired("git repository")),

			huh.NewInput().
				Title("Enter the registry to push the image to").
				Value(&registry).
				Placeholder("public.ecr.aws/repositoryid/repo").
				Validate(validateRequired("registry url")),

			huh.NewInput().
				Title("Enter the region of the ECR registry").
				Value(&region).
				Placeholder("us-east-1").
				Validate(validateRequired("region")),

			huh.NewSelect[string]().
				Title("Choose the architecture of the image").
				Options(
					huh.NewOption("amd64", "amd64"),
					huh.NewOption("arm64", "arm64"),
				).
				Value(&arch),

			huh.NewInput().
				Title("Enter the Dockerfile path in the repository").
				Placeholder("Dockerfile").
				Value(&dockerfilePath).
				Validate(validateRequired("Dockerfile path")),

			// optional input
			huh.NewInput().
				Title("Enter the remote builder address where buildkitd is running (optional)").
				Value(&remoteBuilderAddr).
				Placeholder("tcp://1.2.3.4:1234 or ssh://user@host").
				Validate(validateProtocol("remote builder address", []string{"ssh://", "tcp://"}, true)),
		),
	)

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}

	return PromptResults{
		Repo:                 repo,
		RegistryURL:          registry,
		Region:               region,
		Arch:                 arch,
		DockerfilePath:       dockerfilePath,
		RemoteBuilderAddress: remoteBuilderAddr,
	}
}
