// Package cfgcascade provides a generic, layered configuration cascade.
//
// A Cascade resolves configuration by loading values from ranked providers
// (least-specific first), merging them with a caller-supplied function, and
// tracking which providers contributed to the final value.
//
// Providers that return os.ErrNotExist are silently skipped (source not
// present). Any other error is recorded but does not stop resolution.
package cfgcascade

import (
	"errors"
	"os"
	"sort"
)

// Provider loads a config value from a single source.
type Provider[T any] interface {
	// Load returns a value from this source. Return an error wrapping
	// os.ErrNotExist to signal that the source is absent (graceful skip).
	Load(getenv func(string) string) (T, error)

	// Name returns a human-readable name for this provider (e.g. "user-config",
	// "env-vars").
	Name() string
}

// Layer is a ranked config source. Lower Rank values are less specific
// (loaded first, overridden by higher ranks).
type Layer[T any] struct {
	Rank     int
	Provider Provider[T]
}

// ProviderError records a provider that failed with a non-ErrNotExist error.
type ProviderError struct {
	Name string
	Err  error
}

func (pe ProviderError) Error() string {
	return pe.Name + ": " + pe.Err.Error()
}

func (pe ProviderError) Unwrap() error {
	return pe.Err
}

// ResolvedValue holds the merged result and provenance metadata.
type ResolvedValue[T any] struct {
	// Value is the final merged configuration.
	Value T

	// Sources lists the names of providers that contributed, ordered from
	// most-specific (highest rank) to least-specific.
	Sources []string

	// Errors lists providers that returned non-ErrNotExist errors.
	Errors []ProviderError
}

// Cascade merges config layers in rank order using a provided merge function.
type Cascade[T any] struct {
	// Layers are the ranked config sources. They do not need to be pre-sorted;
	// Resolve sorts them by Rank before processing.
	Layers []Layer[T]

	// MergeFn combines two values. It receives (base, overlay) where overlay
	// comes from the higher-ranked (more specific) layer. The returned value
	// becomes the new base for subsequent merges.
	MergeFn func(base, overlay T) T
}

// Resolve loads all layers in rank order (ascending), skips providers that
// return os.ErrNotExist, records real errors, and merges using MergeFn.
//
// If no provider succeeds, Value will be the zero value of T with an empty
// Sources slice.
func (c *Cascade[T]) Resolve(getenv func(string) string) *ResolvedValue[T] {
	sorted := make([]Layer[T], len(c.Layers))
	copy(sorted, c.Layers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Rank < sorted[j].Rank
	})

	rv := &ResolvedValue[T]{}
	haveBase := false

	for _, layer := range sorted {
		val, err := layer.Provider.Load(getenv)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Source absent — graceful skip.
				continue
			}
			rv.Errors = append(rv.Errors, ProviderError{
				Name: layer.Provider.Name(),
				Err:  err,
			})
			continue
		}

		if !haveBase {
			rv.Value = val
			haveBase = true
		} else {
			rv.Value = c.MergeFn(rv.Value, val)
		}
		rv.Sources = append(rv.Sources, layer.Provider.Name())
	}

	// Reverse sources so most-specific is first.
	for i, j := 0, len(rv.Sources)-1; i < j; i, j = i+1, j-1 {
		rv.Sources[i], rv.Sources[j] = rv.Sources[j], rv.Sources[i]
	}

	return rv
}
