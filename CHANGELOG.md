## [1.4.0] - 2026-03-27

### 🚀 Features

- Add cfgcascade package for hierarchical config resolution

### 📚 Documentation

- Document getenv nil fallback in EnvProvider.Load
## [1.3.0] - 2026-03-26

### 🚀 Features

- Add OpenFile method to FileSystem interface

### 📚 Documentation

- *(release)* Update changelog for v1.3.0
## [1.2.0] - 2026-03-17

### 🚀 Features

- Add AppendFile to FileSystem interface and Runtime
- Add mylog tests (86% coverage) and fix TestHandler entry sharing

### 🐛 Bug Fixes

- Wrap actual error instead of os.ErrNotExist in NewAppPaths
- Remove broken goreleaser config and release workflow

### 📚 Documentation

- Update CLAUDE.md and README.md to describe Runtime-based architecture
- Add package-level godoc to all packages
- *(release)* Update changelog for v1.2.0
## [1.1.0] - 2026-03-04

### 🚀 Features

- Add ProcessInfo to Runtime for lock ownership tracking

### 📚 Documentation

- *(release)* Update changelog for v1.1.0
## [1.0.2] - 2026-02-26

### 🐛 Bug Fixes

- Downgrade git rev-parse fallback log level for non-git directories

### 📚 Documentation

- *(release)* Update changelog for v1.0.2
## [1.0.1] - 2026-02-24

### 📚 Documentation

- Update README and refactor runtime dependency wiring
- *(release)* Update changelog for v1.0.1
## [1.0.0] - 2026-02-23

### 🚀 Features

- [**breaking**] Refactor Runtime to encapsulate dependencies and add filesystem jail support

### 📚 Documentation

- *(release)* Update changelog for v1.0.0
## [0.4.0] - 2025-12-08

### 🚀 Features

- Implement GetTempDir and ensure ExpandPath usage in OsEnv

### 📚 Documentation

- *(release)* Update changelog for v0.4.0
## [0.3.0] - 2025-12-07

### 🚀 Features

- Simplify app context initialization and improve test logging
- Add Glob function to Env interfaces and improve filesystem testing

### 📚 Documentation

- *(release)* Update changelog for v0.3.0
## [0.2.1] - 2025-11-16

### 🐛 Bug Fixes

- Remove redundant error logging in AtomicWriteFile

### 📚 Documentation

- *(release)* Update changelog for v0.2.1
## [0.2.0] - 2025-11-10

### 🚀 Features

- Add RunWithIO method and expand test coverage

### 📚 Documentation

- *(release)* Update changelog for v0.2.0
## [0.1.2] - 2025-11-10

### 🐛 Bug Fixes

- Resolve AtomicWriteFile jail path handling in TestEnv

### 📚 Documentation

- *(release)* Update changelog for v0.1.1
- *(release)* Update changelog for v0.1.2
## [0.1.1] - 2025-11-09

### 🐛 Bug Fixes

- Make following symlinks not the default

### 📚 Documentation

- Update README with package organization and examples
- *(changelog)* Update changelog for 0.1.1
## [0.1.0] - 2025-11-08

### 🚀 Features

- Add std package with env, clock, fs, and user helpers
- Add logger, atomic file write, and test env helpers
- Add context helpers and extend Env API
- Add ExpandPath helper to expand leading tilde
- Expand Env interface and add path and env utilities
- Add hasher, testutils, Edit helper and bump deps
- Add project package and improve test utilities
- Add Stream to Env and fixture stdio support
- Add single-process test Harness and improve TestEnv
- Add experimental Stream interface for I/O abstraction
- Add filepath jail utilities for sandboxing paths

### 🐛 Bug Fixes

- Make clock context helpers robust and add tests

### 📚 Documentation

- Add README with overview and usage examples
