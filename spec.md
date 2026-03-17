Specification: Single-Process Harness (integrated with Fixture) Purpose

- Provide a small, declarable test harness that runs exactly one unit of work (a
  "process") per harness.
- Preserve the existing Fixture for logger/clock/hasher/jail/test-env behavior
  while giving each process an isolated TestEnv derived from the Fixture.
- Support running either:
  - a generic Runner (ProcFunc-style), or
  - a cobra.Command (wrapped via CobraRunner), and allow wiring harness stdout
    to another harness stdin for pipelines.
- Provide easy test-time control of stdin/stdout/stderr, argument passing,
  capture, and piping.

Goals / constraints

- One Harness == one process.
- Per-process environment must be isolated but must preserve Fixture context
  values (logger/clock/hasher).
- Streams (stdin/stdout/stderr) are explicit and test-controlled.
  std.StreamFromContext must reflect the harness streams inside the process.
- Piping requires concurrent execution of the connected harnesses.
- Clear ownership and close/EOF semantics: harness-owned writers must be closed
  on process exit to let downstream readers observe EOF. Tests that get writers
  must close them when they are done.

Primary types and adapters

- Runner interface
  - Signature: Run(ctx context.Context, stream std.Stream, args []string) error
  - Implementations:
    - RunnerFunc(func(ctx context.Context, stream std.Stream, args []string)
      error) — convenience adapter to allow simple functions to be Runners.
    - CobraRunner(cmd *cobra.Command) Runner — adapter that sets
      cmd.SetIn/SetOut/SetErr, cmd.SetArgs(args), cmd.SetContext(ctx) and calls
      cmd.ExecuteContext(ctx).

- Harness (single-process) — constructed using NewHarnessFromFixture(t
  *testing.T, f *Fixture, r Runner)
  - Fields:
    - t *testing.T
    - f *Fixture
    - run Runner
    - Args []string
    - inReader io.ReadCloser
    - inWriter io.WriteCloser
    - outReader io.ReadCloser
    - outWriter io.WriteCloser
    - outBuf *bytes.Buffer
    - errBuf *bytes.Buffer
    - env *std.TestEnv (cloned per run)
    - mu sync.Mutex

Public methods and behaviors

- NewHarnessFromFixture(t, f, r) *Harness
  - Create harness bound to a Fixture and a Runner.

- StdinWriter() io.WriteCloser
  - Creates an io.Pipe and returns the writer for test code to write into the
    harness stdin.
  - Sets harness.inReader to the pipe reader and harness.inWriter to the pipe
    writer.
  - Tests MUST close the returned writer when done to signal EOF to the process.
  - Fails test if stdin already configured.

- StdoutPipe() io.ReadCloser
  - Creates an io.Pipe (writer used internally as harness stdout) and returns
    the reader.
  - Consecutive calls return the same reader.
  - Used to pass harness stdout into another harness via SetStdinFromReader.

- SetStdinFromReader(r io.Reader)
  - Sets harness.inReader using the provided reader (wrapped as io.NopCloser if
    needed).
  - Used to accept another harness's StdoutPipe() stream.
  - Fails test if stdin already configured.

- CaptureStdout() *bytes.Buffer
  - Configure capture of stdout; returns buffer with captured bytes.
  - If stdout is piped and capture requested, harness will use io.MultiWriter so
    downstream and capture both receive output.

- CaptureStderr() *bytes.Buffer
  - Configure capture of stderr.

- Run(ctx context.Context) error
  - High-level lifecycle:
    1. Clone per-run TestEnv from Fixture: env := f.env.Clone(); store into
       harness.env.
    2. Prepare streams:
       - stdin: default os.Stdin unless harness.inReader set (then use that and
         set env.SetStdioPiped(true)).
       - stdout: default os.Stdout unless harness.outWriter set (then use that).
       - If CaptureStdout() was called, apply io.MultiWriter(outWriter, outBuf)
         (or just outBuf if no outWriter).
       - stderr: default os.Stderr or errBuf when captured.
    3. Install streams into the cloned env:
       - env.SetStdio(in)
       - env.SetStdout(out)
       - env.SetStderr(errw)
    4. Build std.Stream { In: in, Out: out, Err: errw, IsPiped: (inReader !=
       nil), IsTTY: false }
    5. Create run context: procCtx := std.WithEnv(f.Context(), env) (this
       preserves Fixture context values but overrides Env).
    6. Call runner.Run(procCtx, stream, args).
    7. After runner returns close any harness-owned writer(s): close(outWriter)
       and close(inReader) (if created).
    8. Return error from runner.Run (propagate directly).
  - Notes:
    - For cobra commands, CobraRunner ensures cmd.SetArgs(args).
    - Run is synchronous; when piping, caller must start both harnesses
      concurrently (goroutines).

Stream ownership and close semantics

- If harness created a pipe writer (outWriter) for its stdout, Harness.Run
  closes outWriter after the run completes to signal EOF to downstream readers.
- If harness created a pipe reader for stdin (via StdinWriter), Harness.Run
  closes that reader at the end to free resources; tests should close writers
  they obtain.
- If harness.inReader is provided from another source (e.g., SetStdinFromReader
  with another harness's StdoutPipe), the producing harness (the one that
  created the writer) is responsible for closing the writer; the consumer
  harness should not close the reader provided externally.

Argument passing

- Harness.Args []string is the way to attach command-line-like arguments to the
  run.
- For cobra commands: CobraRunner calls cmd.SetArgs(h.Args) before
  ExecuteContext so cobra behaves as if invoked with those arguments.
- For RunnerFunc: args are passed as the args parameter to Runner.Run.

Concurrency / piping

- io.Pipe requires both reader and writer live concurrently. For pipelines (A ->
  B):
  - The test must call hA.StdoutPipe() and pass the returned reader to
    hB.SetStdinFromReader(...).
  - Then start both runs concurrently in separate goroutines:
    - go hA.Run(f.Context())
    - go hB.Run(f.Context())
  - Collect errors from both runs and assert results.
- Deadlocks: tests should consider timeouts on contexts (ctx with timeout) to
  avoid hanging tests.

Compatibility with std.StreamFromContext

- Harness.Run uses env.SetStdio(in)/SetStdout(out)/SetStderr(errw) on the cloned
  TestEnv and constructs procCtx := std.WithEnv(f.Context(), env). Any code that
  calls std.StreamFromContext(procCtx) will see the harness-provided streams.

Capture and teeing

- If both piping and capture are requested for a harness's stdout, harness will
  use io.MultiWriter so both the pipe writer and the capture buffer receive the
  same bytes.
- Same applies for stderr when capture is requested.

Error handling

- Harness.Run returns the error returned by runner.Run (first-level
  propagation). For pipelines, caller collects errors from each harness
  goroutine and handles them (e.g., fail test on any error).
- For unexpected test misuse (e.g., configuring stdin twice), harness methods
  call t.Fatalf to fail fast.

Recommended test patterns

- Single command:
  - h := NewHarnessFromFixture(t, f, CobraRunner(cmd))
  - h.Args = [...]
  - out := h.CaptureStdout()
  - require.NoError(t, h.Run(f.Context()))
  - assert on out.String()
- Piping two harnesses:
  - hA := NewHarnessFromFixture(t, f, runnerA)
  - hB := NewHarnessFromFixture(t, f, runnerB)
  - r := hA.StdoutPipe()
  - hB.SetStdinFromReader(r)
  - out := hB.CaptureStdout()
  - w := hA.StdinWriter() (if needed); write input; close(w)
  - go errCh <- hA.Run(f.Context())
  - go errCh <- hB.Run(f.Context())
  - assert no errors and inspect out

Edge cases & recommendations

- Always close writers returned by StdinWriter() to avoid blocking readers
  indefinitely.
- Use context timeouts for runs that may deadlock.
- If you need a producer to feed multiple consumers, extend harness to record
  multiple outWriters and use io.MultiWriter for the producer output; ensure
  closing semantics close all writers.
- If you need per-harness TTY behavior, provide SetTTY/WithTTY before Run to set
  env.SetTTY(true) on the cloned env.
- Consider adding convenience helpers (ChainTwo, RunAndCapture, RunWithTimeout)
  in the harness package for common patterns.

Optional extensions

- ChainTwo(hA, hB) helper that wires StdoutPipe and SetStdinFromReader, runs
  both and returns captured output (internal goroutine orchestration and error
  aggregation).
- Support for multiple writer destinations for one producer (fan-out).
- Support for test-level timeouts and automatic cancellation propagation.

Method signatures summary (reference)

- NewHarnessFromFixture(t *testing.T, f *Fixture, r Runner) *Harness
- (h *Harness) StdinWriter() io.WriteCloser
- (h *Harness) StdoutPipe() io.ReadCloser
- (h *Harness) SetStdinFromReader(r io.Reader)
- (h *Harness) CaptureStdout() *bytes.Buffer
- (h *Harness) CaptureStderr() *bytes.Buffer
- (h *Harness) Run(ctx context.Context) error
- CobraRunner(cmd *cobra.Command) Runner
- RunnerFunc(func(ctx context.Context, s std.Stream, args []string) error)
  implements Runner

This specification should let you implement the harness or verify the existing
PoC against a clear contract: single-process-per-harness, Fixture-based context,
explicit stream wiring, arguments, capture, and predictable lifecycle/close
semantics.
