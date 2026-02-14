package appctx_test

import (
	"embed"
	"testing"

	testutils "github.com/jlrickert/cli-toolkit/sandbox"
)

//go:embed all:data/**
var testdata embed.FS

func NewSandbox(t *testing.T, opts ...testutils.SandboxOption) *testutils.Sandbox {
	return testutils.NewSandbox(t,
		&testutils.SandboxOptions{
			Data: testdata,
			Home: "/home/testuser",
			User: "testuser",
		}, opts...)
}
