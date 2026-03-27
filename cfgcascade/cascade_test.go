package cfgcascade_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/jlrickert/cli-toolkit/cfgcascade"
	"github.com/stretchr/testify/require"
)

// testConfig is a simple config struct used across tests.
type testConfig struct {
	Name     string
	LogLevel string
	Port     int
}

// merge overlays non-zero fields from overlay onto base.
func merge(base, overlay testConfig) testConfig {
	if overlay.Name != "" {
		base.Name = overlay.Name
	}
	if overlay.LogLevel != "" {
		base.LogLevel = overlay.LogLevel
	}
	if overlay.Port != 0 {
		base.Port = overlay.Port
	}
	return base
}

// nilGetenv simulates an environment with no variables set.
func nilGetenv(_ string) string { return "" }

func TestCascade_AllProvidersSucceed(t *testing.T) {
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "app", LogLevel: "info", Port: 8080},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "user-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{LogLevel: "debug"}, nil
					},
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "project-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Port: 9090}, nil
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, "app", rv.Value.Name, "Name from defaults")
	require.Equal(t, "debug", rv.Value.LogLevel, "LogLevel overridden by user-config")
	require.Equal(t, 9090, rv.Value.Port, "Port overridden by project-config")

	// Sources: most-specific first.
	require.Equal(t, []string{"project-config", "user-config", "defaults"}, rv.Sources)
	require.Empty(t, rv.Errors)
}

func TestCascade_ErrNotExistSkipped(t *testing.T) {
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "app", LogLevel: "info", Port: 8080},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.MissingProvider[testConfig]{
					ProviderName: "user-config",
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "project-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Port: 3000}, nil
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, "app", rv.Value.Name)
	require.Equal(t, "info", rv.Value.LogLevel, "user-config skipped, defaults preserved")
	require.Equal(t, 3000, rv.Value.Port, "project-config applied")
	require.Equal(t, []string{"project-config", "defaults"}, rv.Sources)
	require.Empty(t, rv.Errors, "ErrNotExist should not appear in errors")
}

func TestCascade_RealErrorRecorded(t *testing.T) {
	corruptErr := fmt.Errorf("corrupt YAML at line 3")
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "app", LogLevel: "info", Port: 8080},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "user-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						var zero testConfig
						return zero, corruptErr
					},
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "project-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Port: 9090}, nil
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	// Defaults and project-config contribute; user-config errored.
	require.Equal(t, "app", rv.Value.Name)
	require.Equal(t, "info", rv.Value.LogLevel, "user-config errored, defaults preserved")
	require.Equal(t, 9090, rv.Value.Port)
	require.Equal(t, []string{"project-config", "defaults"}, rv.Sources)

	require.Len(t, rv.Errors, 1)
	require.Equal(t, "user-config", rv.Errors[0].Name)
	require.ErrorIs(t, rv.Errors[0].Err, corruptErr)
}

func TestCascade_RankOrder(t *testing.T) {
	// Layers provided out of order; cascade should sort by rank.
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "high",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Name: "high"}, nil
					},
				},
			},
			{
				Rank: 0,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "low",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Name: "low"}, nil
					},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "mid",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Name: "mid"}, nil
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	// "high" wins because it has the highest rank and overrides Name.
	require.Equal(t, "high", rv.Value.Name)
	require.Equal(t, []string{"high", "mid", "low"}, rv.Sources)
}

func TestCascade_MergeFnCalledInOrder(t *testing.T) {
	// Track merge call order to verify least-specific first.
	var calls []string
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "tier-0",
					Default:      testConfig{Name: "zero"},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "tier-10",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Name: "ten"}, nil
					},
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "tier-20",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{Name: "twenty"}, nil
					},
				},
			},
		},
		MergeFn: func(base, overlay testConfig) testConfig {
			calls = append(calls, fmt.Sprintf("merge(%s,%s)", base.Name, overlay.Name))
			return merge(base, overlay)
		},
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, "twenty", rv.Value.Name)
	require.Equal(t, []string{
		"merge(zero,ten)",
		"merge(ten,twenty)",
	}, calls, "merge should be called least-specific to most-specific")
}

func TestCascade_NoProviders(t *testing.T) {
	c := &cfgcascade.Cascade[testConfig]{
		Layers:  nil,
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, testConfig{}, rv.Value, "zero value when no providers")
	require.Empty(t, rv.Sources)
	require.Empty(t, rv.Errors)
}

func TestCascade_AllProvidersMissing(t *testing.T) {
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{Rank: 0, Provider: &cfgcascade.MissingProvider[testConfig]{ProviderName: "a"}},
			{Rank: 10, Provider: &cfgcascade.MissingProvider[testConfig]{ProviderName: "b"}},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, testConfig{}, rv.Value)
	require.Empty(t, rv.Sources)
	require.Empty(t, rv.Errors)
}

func TestCascade_SingleProvider(t *testing.T) {
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "only",
					Default:      testConfig{Name: "solo", Port: 1234},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, testConfig{Name: "solo", Port: 1234}, rv.Value)
	require.Equal(t, []string{"only"}, rv.Sources)
}

func TestCascade_MultipleErrors(t *testing.T) {
	err1 := fmt.Errorf("error 1")
	err2 := fmt.Errorf("error 2")
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "app"},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "broken-a",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{}, err1
					},
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "broken-b",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{}, err2
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, "app", rv.Value.Name)
	require.Equal(t, []string{"defaults"}, rv.Sources)
	require.Len(t, rv.Errors, 2)
	require.Equal(t, "broken-a", rv.Errors[0].Name)
	require.Equal(t, "broken-b", rv.Errors[1].Name)
}

func TestCascade_ErrNotExistWrapped(t *testing.T) {
	// Verify that wrapped os.ErrNotExist is also treated as missing.
	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "app"},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "wrapped-missing",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{}, fmt.Errorf("config file: %w", os.ErrNotExist)
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(nilGetenv)

	require.Equal(t, "app", rv.Value.Name)
	require.Equal(t, []string{"defaults"}, rv.Sources)
	require.Empty(t, rv.Errors, "wrapped ErrNotExist should be treated as missing")
}

func TestProviderError_ErrorString(t *testing.T) {
	pe := cfgcascade.ProviderError{
		Name: "user-config",
		Err:  fmt.Errorf("invalid YAML"),
	}
	require.Equal(t, "user-config: invalid YAML", pe.Error())
}

func TestProviderError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	pe := cfgcascade.ProviderError{
		Name: "test",
		Err:  inner,
	}
	require.ErrorIs(t, pe, inner)
}

// --- EnvProvider tests ---

func TestEnvProvider_WithPrefix(t *testing.T) {
	env := map[string]string{
		"TAP_DEFAULT_KEG": "home",
		"TAP_LOG_LEVEL":   "debug",
		"UNRELATED":       "ignored",
	}
	getenv := func(key string) string { return env[key] }

	p := &cfgcascade.EnvProvider{
		ProviderName: "env",
		Prefix:       "TAP_",
		Keys:         []string{"DEFAULT_KEG", "LOG_LEVEL", "MISSING_KEY"},
	}

	result, err := p.Load(getenv)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"default_keg": "home",
		"log_level":   "debug",
	}, result)
}

func TestEnvProvider_NoMatchingVars(t *testing.T) {
	getenv := func(_ string) string { return "" }

	p := &cfgcascade.EnvProvider{
		ProviderName: "env",
		Prefix:       "TAP_",
		Keys:         []string{"DEFAULT_KEG", "LOG_LEVEL"},
	}

	_, err := p.Load(getenv)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnvProvider_EmptyKeys(t *testing.T) {
	p := &cfgcascade.EnvProvider{
		ProviderName: "env",
		Prefix:       "TAP_",
		Keys:         nil,
	}

	_, err := p.Load(nilGetenv)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnvProvider_Name(t *testing.T) {
	p := &cfgcascade.EnvProvider{ProviderName: "test-env"}
	require.Equal(t, "test-env", p.Name())
}

func TestEnvProvider_NilGetenvFallback(t *testing.T) {
	// When getenv is nil, EnvProvider should fall back to os.Getenv.
	// This test just verifies it doesn't panic.
	p := &cfgcascade.EnvProvider{
		ProviderName: "env",
		Prefix:       "CFGCASCADE_TEST_UNLIKELY_PREFIX_",
		Keys:         []string{"SOME_KEY"},
	}

	_, err := p.Load(nil)
	// Expect ErrNotExist since no real env vars match the prefix.
	require.ErrorIs(t, err, os.ErrNotExist)
}

// --- DefaultProvider tests ---

func TestDefaultProvider_AlwaysSucceeds(t *testing.T) {
	p := &cfgcascade.DefaultProvider[testConfig]{
		ProviderName: "defaults",
		Default:      testConfig{Name: "default-app", Port: 8080},
	}

	val, err := p.Load(nilGetenv)
	require.NoError(t, err)
	require.Equal(t, "default-app", val.Name)
	require.Equal(t, 8080, val.Port)
	require.Equal(t, "defaults", p.Name())
}

// --- FuncProvider tests ---

func TestFuncProvider_PassesGetenv(t *testing.T) {
	env := map[string]string{"MY_VAR": "hello"}
	getenv := func(key string) string { return env[key] }

	p := &cfgcascade.FuncProvider[testConfig]{
		ProviderName: "func-env",
		Fn: func(ge func(string) string) (testConfig, error) {
			return testConfig{Name: ge("MY_VAR")}, nil
		},
	}

	val, err := p.Load(getenv)
	require.NoError(t, err)
	require.Equal(t, "hello", val.Name)
}

// --- MissingProvider tests ---

func TestMissingProvider_AlwaysReturnsErrNotExist(t *testing.T) {
	p := &cfgcascade.MissingProvider[testConfig]{ProviderName: "missing"}
	_, err := p.Load(nilGetenv)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Equal(t, "missing", p.Name())
}

// --- Integration: Cascade with EnvProvider ---

func TestCascade_WithEnvProvider(t *testing.T) {
	// Simulate a realistic cascade: defaults -> user file -> env vars.
	env := map[string]string{
		"APP_LOG_LEVEL": "trace",
		"APP_PORT":      "4000",
	}
	getenv := func(key string) string { return env[key] }

	// The env provider returns map[string]string, so we use a FuncProvider
	// that wraps the EnvProvider and maps its output to testConfig.
	envProv := &cfgcascade.EnvProvider{
		ProviderName: "env-raw",
		Prefix:       "APP_",
		Keys:         []string{"LOG_LEVEL", "PORT", "NAME"},
	}

	c := &cfgcascade.Cascade[testConfig]{
		Layers: []cfgcascade.Layer[testConfig]{
			{
				Rank: 0,
				Provider: &cfgcascade.DefaultProvider[testConfig]{
					ProviderName: "defaults",
					Default:      testConfig{Name: "myapp", LogLevel: "info", Port: 8080},
				},
			},
			{
				Rank: 10,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "user-config",
					Fn: func(_ func(string) string) (testConfig, error) {
						return testConfig{LogLevel: "warn"}, nil
					},
				},
			},
			{
				Rank: 20,
				Provider: &cfgcascade.FuncProvider[testConfig]{
					ProviderName: "env-vars",
					Fn: func(ge func(string) string) (testConfig, error) {
						raw, err := envProv.Load(ge)
						if err != nil {
							return testConfig{}, err
						}
						cfg := testConfig{}
						if v, ok := raw["log_level"]; ok {
							cfg.LogLevel = v
						}
						if v, ok := raw["port"]; ok {
							// Simple conversion for test.
							if v == "4000" {
								cfg.Port = 4000
							}
						}
						return cfg, nil
					},
				},
			},
		},
		MergeFn: merge,
	}

	rv := c.Resolve(getenv)

	require.Equal(t, "myapp", rv.Value.Name, "Name from defaults (not overridden)")
	require.Equal(t, "trace", rv.Value.LogLevel, "LogLevel from env-vars (highest rank)")
	require.Equal(t, 4000, rv.Value.Port, "Port from env-vars")
	require.Equal(t, []string{"env-vars", "user-config", "defaults"}, rv.Sources)
	require.Empty(t, rv.Errors)
}
