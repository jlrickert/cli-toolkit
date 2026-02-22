package appctx_test

import (
	"embed"
	"testing"

	testutils "github.com/jlrickert/cli-toolkit/sandbox"
)

//go:embed all:data/**
var testdata embed.FS

func NewSandbox(t *testing.T, opts ...testutils.Option) *testutils.Sandbox {
	return testutils.NewSandbox(t,
		&testutils.Options{
			Data: testdata,
			Home: "/home/testuser",
			User: "testuser",
		}, opts...)
}
