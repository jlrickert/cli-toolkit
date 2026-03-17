# Testing CLI Applications with Sandbox, Process, and Pipeline

## Overview

The `sandbox` package provides three complementary testing utilities for CLI
applications: `Sandbox` for test environment setup, `Process` for running
individual functions in isolation, and `Pipeline` for testing multi-stage
command chains with piped I/O.

## Sandbox: Test Environment Setup

`Sandbox` bundles common test setup including context, runtime, logger, environment
variables, clock, hasher, and an isolated filesystem jail.

### Basic Setup

```go
sandbox := tu.NewSandbox(t, nil)
ctx := sandbox.Context()
rt := sandbox.Runtime()
```

### Configuration Options

Use option functions to customize the sandbox:

```go
sandbox := tu.NewSandbox(t, nil,
  tu.WithEnv("DEBUG", "true"),
  tu.WithEnv("LOG_LEVEL", "info"),
  tu.WithClock(time.Date(2025, 10, 15, 12, 30, 0, 0, time.UTC)),
  tu.WithWd("~/projects"),
)
```

### Context Services

Use `sandbox.Runtime()` for mutable runtime dependencies:

- **Environment**: `sandbox.Runtime().Env()`
- **Filesystem**: `sandbox.Runtime().FS()`
- **Clock**: `sandbox.Runtime().Clock()`
- **Logger**: `sandbox.Runtime().Logger()`
- **Hasher**: `sandbox.Runtime().Hasher()`
- **Stream**: `sandbox.Runtime().Stream()`

### File Operations

Operations are confined to the sandbox jail:

```go
sandbox.MustWriteFile("config.yaml", data, 0o644)
content := sandbox.MustReadFile("config.yaml")
path, _ := sandbox.ResolvePath("~/data/file.txt")
```

### Fixtures

Load test data from embedded files:

```go
sandbox := tu.NewSandbox(t, &tu.SandboxOptions{
  Data: testdata,
}, tu.WithFixture("example", "~/fixtures/example"))
```

## Process: Individual Function Execution

`Process` runs a `Runner` function in isolation with configurable I/O streams
and supports piping between processes.

### Runner Function Signature

```go
type Runner func(ctx context.Context, rt *toolkit.Runtime) (int, error)
```

The stream is available as `rt.Stream()` (`In`, `Out`, `Err`, `IsPiped`, `IsTTY`).

### Simple Execution

```go
runner := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
  _, _ = fmt.Fprintln(rt.Stream().Out, "hello, world")
  return 0, nil
}

process := tu.NewProcess(runner, false)
result := process.Run(context.Background(), sandbox.Runtime())

// result.Err, result.ExitCode, result.Stdout, result.Stderr
```

### Capturing Output

```go
process := tu.NewProcess(runner, false)
outBuf := process.CaptureStdout()
errBuf := process.CaptureStderr()
result := process.Run(t.Context(), sandbox.Runtime())

assert.Equal(t, expected, outBuf.String())
```

### Continuous Input

Write to process stdin while it runs:

```go
process := tu.NewProcess(consumer, false)

go func() {
  for i := 0; i < 20; i++ {
    fmt.Fprintf(process, "line-%d\n", i)
    time.Sleep(5 * time.Millisecond)
  }
  _ = process.Close()
}()

result := process.Run(t.Context(), sandbox.Runtime())
```

### Piping Between Processes

Connect producer output to consumer input:

```go
producer := tu.NewProcess(producerRunner, false)
consumer := tu.NewProcess(consumerRunner, false)

// Wire producer stdout to consumer stdin
r := producer.StdoutPipe()
consumer.SetStdin(r)

// Run concurrently
var wg sync.WaitGroup
errCh := make(chan error, 2)

wg.Go(func() {
  res := producer.Run(ctx, sandbox.Runtime())
  errCh <- res.Err
})

wg.Go(func() {
  res := consumer.Run(ctx, sandbox.Runtime())
  errCh <- res.Err
})

wg.Wait()
```

## Pipeline: Multi-Stage Command Chains

`Pipeline` manages sequential stage execution with automatic piping of stdout to
stdin between stages.

### Basic Pipeline

```go
pipeline := tu.NewPipeline(
  tu.Stage("producer", producerRunner),
  tu.Stage("consumer", consumerRunner),
)

result := pipeline.Run(t.Context(), sandbox.Runtime())
```

### Stages and Wiring

Stages are connected automatically:

- Stage 1 stdout -> Stage 2 stdin
- Stage 2 stdout -> Stage 3 stdin
- Final stage output captured in result

```go
pipeline := tu.NewPipeline(
  tu.Stage("filter", filterRunner),
  tu.Stage("transform", transformRunner),
  tu.Stage("aggregate", aggregateRunner),
)

result := pipeline.Run(t.Context(), sandbox.Runtime())
assert.Equal(t, expected, string(result.Stdout))
```

### Output Capture

```go
pipeline := tu.NewPipeline(
  tu.Stage("producer", producerRunner),
  tu.Stage("consumer", consumerRunner),
)

outBuf := pipeline.CaptureStdout()
errBuf := pipeline.CaptureStderr()

result := pipeline.Run(t.Context(), sandbox.Runtime())

// Output available from both buffers and result
assert.Equal(t, outBuf.String(), string(result.Stdout))
```

### Timeouts

Execute pipeline with a deadline:

```go
result := pipeline.RunWithTimeout(t.Context(), sandbox.Runtime(), 5*time.Second)
if errors.Is(result.Err, context.DeadlineExceeded) {
  t.Fatal("pipeline timeout")
}
```

## Common Patterns

### Testing Data Transformation

```go
producer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
  lines := []string{"alpha", "beta", "gamma"}
  for _, line := range lines {
    _, _ = fmt.Fprintln(rt.Stream().Out, line)
  }
  return 0, nil
}

transformer := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
  sc := bufio.NewScanner(rt.Stream().In)
  for sc.Scan() {
    _, _ = fmt.Fprintln(rt.Stream().Out, strings.ToUpper(sc.Text()))
  }
  return 0, sc.Err()
}

pipeline := tu.NewPipeline(
  tu.Stage("producer", producer),
  tu.Stage("transformer", transformer),
)

result := pipeline.Run(t.Context(), sandbox.Runtime())
assert.Equal(t, "ALPHA\nBETA\nGAMMA\n", string(result.Stdout))
```

### Testing Error Handling

```go
failingStage := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
  return 1, fmt.Errorf("stage failed")
}

pipeline := tu.NewPipeline(
  tu.Stage("failing", failingStage),
)

result := pipeline.Run(t.Context(), sandbox.Runtime())
require.Error(t, result.Err)
assert.Equal(t, 1, result.ExitCode)
```

### Testing with Environment Context

```go
sandbox := tu.NewSandbox(t, nil,
  tu.WithEnv("CONFIG_PATH", "~/.config"),
)

runner := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
  configPath := rt.Get("CONFIG_PATH")
  // Use config path
  return 0, nil
}

process := tu.NewProcess(runner, false)
result := process.Run(sandbox.Context(), sandbox.Runtime())
```
