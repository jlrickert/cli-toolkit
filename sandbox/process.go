package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jlrickert/cli-toolkit/toolkit"
)

// Runner executes a unit of work using explicit runtime dependencies.
type Runner func(ctx context.Context, rt *toolkit.Runtime) (int, error)

// ProcessResult holds the outcome of process execution.
type ProcessResult struct {
	Err      error
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Process manages execution of a Runner with configurable streams.
type Process struct {
	args  []string
	isTTY bool

	runner Runner

	in  io.Reader
	out io.Writer
	err io.Writer

	stdoutPipe *io.PipeReader
	stdoutW    *io.PipeWriter
	stderrPipe *io.PipeReader
	stderrW    *io.PipeWriter

	stdinPipe *io.PipeReader
	stdinW    *io.PipeWriter

	outBuf *bytes.Buffer
	errBuf *bytes.Buffer

	mu sync.Mutex
}

// NewProcess constructs a Process bound to a Runner function.
func NewProcess(fn Runner, isTTY bool) *Process {
	return &Process{runner: fn, isTTY: isTTY}
}

// NewProducer constructs a Process that emits the provided lines to stdout.
func NewProducer(interval time.Duration, lines []string) *Process {
	runner := func(ctx context.Context, rt *toolkit.Runtime) (int, error) {
		s := rt.Stream
		for _, l := range lines {
			fmt.Fprintln(s.Out, l)
			time.Sleep(interval)
		}
		return 0, nil
	}

	return NewProcess(runner, false)
}

// StdoutPipe returns a reader connected to the process stdout.
func (p *Process) StdoutPipe() io.Reader {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdoutPipe == nil {
		p.stdoutPipe, p.stdoutW = io.Pipe()
	}
	return p.stdoutPipe
}

// CaptureStdout configures stdout capture and returns the buffer.
func (p *Process) CaptureStdout() *bytes.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.outBuf == nil {
		p.outBuf = &bytes.Buffer{}
	}
	return p.outBuf
}

// StderrPipe returns a reader connected to the process stderr.
func (p *Process) StderrPipe() io.Reader {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stderrPipe == nil {
		p.stderrPipe, p.stderrW = io.Pipe()
	}
	return p.stderrPipe
}

// CaptureStderr configures stderr capture and returns the buffer.
func (p *Process) CaptureStderr() *bytes.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.errBuf == nil {
		p.errBuf = &bytes.Buffer{}
	}
	return p.errBuf
}

// SetStdin sets the input stream for the process.
func (p *Process) SetStdin(r io.Reader) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.in = r
}

// SetStderr sets the error stream for the process.
func (p *Process) SetStderr(w io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = w
}

// SetStdout sets the output stream for the process.
func (p *Process) SetStdout(w io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.out = w
}

// SetArgs sets the command-line arguments for the process.
func (p *Process) SetArgs(args []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.args = args
}

// Write writes data to the process stdin.
func (p *Process) Write(b []byte) (int, error) {
	p.mu.Lock()
	if p.stdinW == nil {
		p.stdinPipe, p.stdinW = io.Pipe()
		p.in = p.stdinPipe
	}
	w := p.stdinW
	p.mu.Unlock()
	return w.Write(b)
}

// Close closes the process stdin writer.
func (p *Process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdinW != nil {
		return p.stdinW.Close()
	}
	return nil
}

// Run executes the process runner synchronously using a cloned runtime.
func (p *Process) Run(ctx context.Context, rt *toolkit.Runtime) *ProcessResult {
	result := &ProcessResult{}

	if p.runner == nil {
		result.Err = fmt.Errorf("Run: no runner configured")
		result.ExitCode = 1
		return result
	}
	if rt == nil {
		result.Err = fmt.Errorf("Run: runtime is nil")
		result.ExitCode = 1
		return result
	}

	procRt := rt.Clone()
	if procRt == nil {
		result.Err = fmt.Errorf("Run: failed to clone runtime")
		result.ExitCode = 1
		return result
	}

	p.mu.Lock()

	in := p.in
	if in == nil {
		if p.stdinPipe == nil || p.stdinW == nil {
			p.stdinPipe, p.stdinW = io.Pipe()
		}
		in = p.stdinPipe
		p.in = in
	}

	out := p.out
	if out == nil {
		if p.outBuf != nil {
			out = p.outBuf
		} else if p.stdoutW != nil {
			out = p.stdoutW
		} else {
			out = &bytes.Buffer{}
			p.outBuf = out.(*bytes.Buffer)
		}
	}

	errOut := p.err
	if errOut == nil {
		if p.errBuf != nil {
			errOut = p.errBuf
		} else if p.stderrW != nil {
			errOut = p.stderrW
		} else {
			errOut = &bytes.Buffer{}
			p.errBuf = errOut.(*bytes.Buffer)
		}
	}

	p.mu.Unlock()

	stream := &toolkit.Stream{
		In:      in,
		Out:     out,
		Err:     errOut,
		IsPiped: in != nil,
		IsTTY:   p.isTTY,
	}
	procRt.Stream = stream

	exitCode, err := p.runner(ctx, procRt)

	p.mu.Lock()
	if p.stdoutW != nil {
		_ = p.stdoutW.Close()
	}
	if p.stderrW != nil {
		_ = p.stderrW.Close()
	}
	if p.stdinW != nil {
		_ = p.stdinW.Close()
	}
	p.mu.Unlock()

	result.Err = err
	result.ExitCode = exitCode

	p.mu.Lock()
	if p.outBuf != nil {
		result.Stdout = p.outBuf.Bytes()
	}
	if p.errBuf != nil {
		result.Stderr = p.errBuf.Bytes()
	}
	p.mu.Unlock()

	return result
}

func (p *Process) RunWithIO(ctx context.Context, rt *toolkit.Runtime, r io.Reader) *ProcessResult {
	p.mu.Lock()
	p.in = r
	p.mu.Unlock()
	return p.Run(ctx, rt)
}
