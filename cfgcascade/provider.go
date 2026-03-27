package cfgcascade

import "os"

// DefaultProvider returns a static default value. It never fails.
type DefaultProvider[T any] struct {
	ProviderName string
	Default      T
}

func (p *DefaultProvider[T]) Load(_ func(string) string) (T, error) {
	return p.Default, nil
}

func (p *DefaultProvider[T]) Name() string {
	return p.ProviderName
}

// FuncProvider wraps a function as a Provider.
type FuncProvider[T any] struct {
	ProviderName string
	Fn           func(getenv func(string) string) (T, error)
}

func (p *FuncProvider[T]) Load(getenv func(string) string) (T, error) {
	return p.Fn(getenv)
}

func (p *FuncProvider[T]) Name() string {
	return p.ProviderName
}

// MissingProvider always returns os.ErrNotExist. Useful for representing an
// optional source that is not configured.
type MissingProvider[T any] struct {
	ProviderName string
}

func (p *MissingProvider[T]) Load(_ func(string) string) (T, error) {
	var zero T
	return zero, os.ErrNotExist
}

func (p *MissingProvider[T]) Name() string {
	return p.ProviderName
}
