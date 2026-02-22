package sandbox_test

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"testing"

	tu "github.com/jlrickert/cli-toolkit/sandbox"
	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipeline_EmptyStages(t *testing.T) {
	t.Parallel()

	pipeline := tu.NewPipeline()
	rt := newProcessRuntime(t)
	result := pipeline.Run(t.Context(), rt)

	require.Error(t, result.Err)
	assert.Equal(t, 1, result.ExitCode)
}

func TestPipeline_SingleStage(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		_, _ = fmt.Fprintln(s.Out, "single stage output")
		return 0, nil
	}

	pipeline := tu.NewPipeline(
		tu.Stage("producer", runner),
	)

	rt := newProcessRuntime(t)
	result := pipeline.Run(t.Context(), rt)

	require.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "single stage output\n", string(result.Stdout))
}

func TestPipeline_TwoStages(t *testing.T) {
	t.Parallel()

	producer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		lines := []string{"alpha", "beta", "gamma"}
		for _, line := range lines {
			_, _ = fmt.Fprintln(s.Out, line)
		}
		return 0, nil
	}

	consumer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		sc := bufio.NewScanner(s.In)
		for sc.Scan() {
			line := sc.Text()
			_, _ = fmt.Fprintln(s.Out, "C:"+strings.ToUpper(line))
		}
		return 0, sc.Err()
	}

	pipeline := tu.NewPipeline(
		tu.Stage("producer", producer),
		tu.Stage("consumer", consumer),
	)

	outBuf := pipeline.CaptureStdout()
	rt := newProcessRuntime(t)
	result := pipeline.Run(t.Context(), rt)

	require.NoError(t, result.Err)
	assert.Equal(t, "C:ALPHA\nC:BETA\nC:GAMMA\n", string(result.Stdout))
	assert.Equal(t, outBuf.String(), string(result.Stdout))
}
