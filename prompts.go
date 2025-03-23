package main

import (
	"fmt"
	"log"
	"runtime"

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

type PromptResult struct {
	Repo        string
	RegistryURL string
	Region      string
	Arch        string
}

func detectArch() string {
	arch := runtime.GOARCH

	if arch == "arm64" || arch == "arm" {
		return "arm64"
	}

	return "amd64"
}

func AskInputs() PromptResult {
	var repo string
	var registry string
	var region string
	var arch string = detectArch()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter the repository to clone (Make sure it has a Dockerfile on the root)").
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
		),
	)

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}

	return PromptResult{
		Repo:        repo,
		RegistryURL: registry,
		Region:      region,
		Arch:        arch,
	}
}
