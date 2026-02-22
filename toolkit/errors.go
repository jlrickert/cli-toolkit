package toolkit

import (
	"errors"

	jailpkg "github.com/jlrickert/cli-toolkit/toolkit/jail"
)

var (
	ErrNoEnvKey      = errors.New("env key missing")
	ErrEscapeAttempt = jailpkg.ErrEscapeAttempt
)
