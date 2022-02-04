package main

import (
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"math/rand"

	"os"
	"path"
	"testing"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func TestExamples(t *testing.T) {
	cwd, _ := os.Getwd()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Quick:       true,
		SkipRefresh: true,
		Dir:         path.Join(cwd, "k8s"),
		Config:      map[string]string{},
		Secrets: map[string]string{
			"node_password": RandomString(16),
		},
	})
}
