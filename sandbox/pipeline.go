package sandbox

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jlrickert/cli-toolkit/toolkit"
)

// PipelineStage represents a single stage in a pipeline.
type PipelineStage struct {
	name    string
	runner  Runner
	process *Process
}

// PipelineResult holds the outcome of pipeline execution.
type PipelineResult struct {
	Err      error
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Pipeline manages execution of multiple stages with piped I/O.
type Pipeline struct {
	stages []*PipelineStage

	outBuf *bytes.Buffer
	errBuf *bytes.Buffer

	mu sync.Mutex
}

// StageOption configures a PipelineStage.
type StageOption func(s *PipelineStage)

// Stage constructs a PipelineStage with the given name and runner.
func Stage(name string, runner Runner) *PipelineStage {
	return &PipelineStage{name: name, runner: runner}
}

// StageWithName constructs a PipelineStage wrapping the provided Process.
func StageWithName(name string, p *Process) *PipelineStage {
	return &PipelineStage{name: name, runner: p.runner, process: p}
}

// NewPipeline constructs a Pipeline with the given stages.
func NewPipeline(stages ...*PipelineStage) *Pipeline {
	return &Pipeline{stages: stages}
}

// CaptureStdout configures stdout capture and returns the buffer.
func (p *Pipeline) CaptureStdout() *bytes.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.outBuf == nil {
		p.outBuf = &bytes.Buffer{}
	}
	return p.outBuf
}

// CaptureStderr configures stderr capture and returns the buffer.
func (p *Pipeline) CaptureStderr() *bytes.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.errBuf == nil {
		p.errBuf = &bytes.Buffer{}
	}
	return p.errBuf
}

// Run executes all stages concurrently with stdout piped stage-to-stage.
func (p *Pipeline) Run(ctx context.Context, rt *toolkit.Runtime) *PipelineResult {
	result := &PipelineResult{}

	if rt == nil {
		result.Err = errors.New("pipeline: runtime is nil")
		result.ExitCode = 1
		return result
	}

	if len(p.stages) == 0 {
		result.Err = errors.New("pipeline: no stages")
		result.ExitCode = 1
		return result
	}

	if p.outBuf == nil {
		p.outBuf = &bytes.Buffer{}
	}
	if p.errBuf == nil {
		p.errBuf = &bytes.Buffer{}
	}
	stages := p.stages

	procs := make([]*Process, len(stages))
	for i, stage := range stages {
		if stage.process != nil {
			procs[i] = stage.process
		} else {
			procs[i] = NewProcess(stage.runner, false)
		}
	}

	for i := 0; i < len(procs)-1; i++ {
		r := procs[i].StdoutPipe()
		procs[i+1].SetStdin(r)
	}

	lastProc := procs[len(procs)-1]
	if p.outBuf != nil {
		lastProc.mu.Lock()
		lastProc.outBuf = p.outBuf
		lastProc.mu.Unlock()
	}
	if p.errBuf != nil {
		lastProc.mu.Lock()
		lastProc.errBuf = p.errBuf
		lastProc.mu.Unlock()
	}

	errCh := make(chan error, len(procs))
	var wg sync.WaitGroup

	for _, h := range procs {
		proc := h
		wg.Go(func() {
			res := proc.Run(ctx, rt)
			errCh <- res.Err
		})
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		result.Err = errors.Join(errs...)
		result.ExitCode = 1
	}

	if p.outBuf != nil {
		result.Stdout = p.outBuf.Bytes()
	}
	if p.errBuf != nil {
		result.Stderr = p.errBuf.Bytes()
	}

	return result
}

// RunWithTimeout executes the pipeline with a deadline.
func (p *Pipeline) RunWithTimeout(ctx context.Context, rt *toolkit.Runtime, timeout time.Duration) *PipelineResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.Run(ctx, rt)
}
