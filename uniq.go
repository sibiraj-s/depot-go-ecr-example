package main

import (
	gonanoid "github.com/matoous/go-nanoid/v2"
)

var defaultAlphabet = "_-0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenerateUniqueId() string {
	id, _ := gonanoid.Generate(defaultAlphabet, 8)
	return id
}
