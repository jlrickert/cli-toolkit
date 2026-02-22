package jail

import "errors"

var (
	ErrEscapeAttempt = errors.New("path escape attempt: operation would access path outside jail")
)
