package main

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
)

func CloneRepo(repo string, workflowId string) string {
	cloneDir := JoinPath(CurrentDir(), "tmp", workflowId)

	_, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:      repo,
		Progress: os.Stdout,
	})

	if err != nil {
		fmt.Printf("Clone failed %v\n", err)
		return ""
	}

	return cloneDir
}
