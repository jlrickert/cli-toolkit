package sandbox_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tu "github.com/jlrickert/cli-toolkit/sandbox"
	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProcessRuntime(t *testing.T) *toolkit.Runtime {
	t.Helper()
	env := toolkit.NewTestEnv(t.TempDir(), "", "")
	rt, err := toolkit.NewRuntime(
		toolkit.WithRuntimeEnv(env),
		toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
		toolkit.WithRuntimeJail(env.GetJail()),
	)
	require.NoError(t, err)
	return rt
}

func TestProcess_Run_NoStdin(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		_, _ = fmt.Fprintln(s.Out, "hello, world")
		_, _ = fmt.Fprintln(s.Err, "Some error!")
		return 0, nil
	}

	h := tu.NewProcess(runner, false)
	rt := newProcessRuntime(t)
	result := h.Run(t.Context(), rt)
	require.NoError(t, result.Err)

	assert.Equal(t, "hello, world\n", string(result.Stdout))
	assert.Equal(t, "Some error!\n", string(result.Stderr))
}

func TestProcess_Pipe_ProducerToConsumer(t *testing.T) {
	t.Parallel()

	producer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		lines := []string{"alpha", "beta", "gamma"}
		for _, l := range lines {
			_, _ = fmt.Fprintln(s.Out, l)
			time.Sleep(5 * time.Millisecond)
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

	hProd := tu.NewProcess(producer, false)
	hCons := tu.NewProcess(consumer, false)

	r := hProd.StdoutPipe()
	hCons.SetStdin(r)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	rt := newProcessRuntime(t)

	wg.Go(func() {
		res := hProd.Run(t.Context(), rt)
		errCh <- res.Err
	})

	wg.Go(func() {
		res := hCons.Run(t.Context(), rt)
		errCh <- res.Err
	})

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	expected := "C:ALPHA\nC:BETA\nC:GAMMA\n"
	assert.Equal(t, expected, hCons.CaptureStdout().String())
}

func TestProcess_ContinuousStdin(t *testing.T) {
	t.Parallel()

	const linesToWrite = 20

	consumer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		sc := bufio.NewScanner(s.In)
		for sc.Scan() {
			line := sc.Text()
			_, _ = fmt.Fprintln(s.Out, "C:"+strings.ToUpper(line))
		}
		return 0, sc.Err()
	}

	h := tu.NewProcess(consumer, false)
	out := h.CaptureStdout()
	rt := newProcessRuntime(t)

	errCh := make(chan error, 1)
	go func() {
		res := h.Run(t.Context(), rt)
		errCh <- res.Err
	}()

	go func() {
		for i := range linesToWrite {
			fmt.Fprintf(h, "line-%d\n", i)
			time.Sleep(5 * time.Millisecond)
		}
		_ = h.Close()
	}()

	err := <-errCh
	require.NoError(t, err)

	var b strings.Builder
	for i := range linesToWrite {
		fmt.Fprintf(&b, "C:LINE-%d\n", i)
	}
	assert.Equal(t, b.String(), out.String())
}

func TestProcess_BufferedStdio(t *testing.T) {
	t.Parallel()

	const linesToWrite = 50

	producer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		w := bufio.NewWriter(s.Out)
		for i := range linesToWrite {
			_, _ = fmt.Fprintf(w, "data-%d\n", i)
		}
		_ = w.Flush()
		return 0, nil
	}

	consumer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		r := bufio.NewReader(s.In)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return 1, err
			}
			_, _ = fmt.Fprint(s.Out, "C:"+strings.TrimSpace(line)+"\n")
		}
		return 0, nil
	}

	hProd := tu.NewProcess(producer, false)
	hCons := tu.NewProcess(consumer, false)

	r := hProd.StdoutPipe()
	hCons.SetStdin(r)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	rt := newProcessRuntime(t)

	wg.Go(func() {
		res := hProd.Run(t.Context(), rt)
		errCh <- res.Err
	})

	wg.Go(func() {
		res := hCons.Run(t.Context(), rt)
		errCh <- res.Err
	})

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	output := hCons.CaptureStdout().String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, linesToWrite, len(lines),
		"expected %d lines but got %d", linesToWrite, len(lines))

	for i := range linesToWrite {
		expected := fmt.Sprintf("C:data-%d", i)
		assert.Equal(t, expected, lines[i])
	}
}

func TestProcess_RunWithIO(t *testing.T) {
	t.Parallel()

	consumer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream()
		sc := bufio.NewScanner(s.In)
		for sc.Scan() {
			line := sc.Text()
			_, _ = fmt.Fprintln(s.Out, strings.ToUpper(line))
		}
		return 0, sc.Err()
	}

	h := tu.NewProcess(consumer, false)
	out := h.CaptureStdout()
	inputData := "line one\nline two\nline three\n"
	inputReader := strings.NewReader(inputData)
	rt := newProcessRuntime(t)

	result := h.RunWithIO(t.Context(), rt, inputReader)
	require.NoError(t, result.Err)
	assert.Equal(t, 0, result.ExitCode)

	expected := "LINE ONE\nLINE TWO\nLINE THREE\n"
	assert.Equal(t, expected, out.String())
	assert.Equal(t, expected, string(result.Stdout))
}
