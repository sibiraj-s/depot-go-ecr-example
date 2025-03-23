package main

import (
	"log"
	"os"
	"path/filepath"
)

func CurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func JoinPath(parts ...string) string {
	return filepath.Join(parts...)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func RelativePath(path string) string {
	rel, _ := filepath.Rel(CurrentDir(), path)
	return rel
}

func RemoveDir(path string) error {
	return os.RemoveAll(path)
}
