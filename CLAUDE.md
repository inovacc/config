# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go library (`github.com/inovacc/config`) for application configuration management. Wraps an internalized fork of Viper (`internal/viper/`) with a higher-level API providing type-safe generic access, validation, env var overrides, sensitive value masking, custom validators, and configuration profiles.

## Build & Test Commands

Uses [Task](https://taskfile.dev) as the task runner:

```bash
task test       # fmt + lint + test + bench (full suite)
task upgrade    # go mod tidy + update deps
```

Individual commands:
```bash
go test -race -p=1 ./...                    # run all tests
go test -race -v -run TestSetServiceConfig  # run a single test
golangci-lint run                           # lint only
golangci-lint fmt                           # format only
```

Tests rely on `testdata/config.yaml` and create temp directories for isolation. Tests run sequentially (`-p=1`) because `globalConfig` is package-level mutable state (protected by `sync.RWMutex`).

## Architecture

**Public API** (`config.go`): Single-file package with global `Config` singleton protected by `sync.RWMutex`. Key functions:
- `InitServiceConfig(v any, path string)` - Load config file, bind service struct, load profile overlay, run validators
- `GetServiceConfig[T]()` - Generic type-safe retrieval of service config
- `GetBaseConfig()` - Returns a **copy** (not pointer) of base config
- `SetEnvPrefix(prefix)` - Enable env var overrides (e.g., `APP_LOGGER_LOGLEVEL`)
- `AddValidator(fn)` - Register custom validation functions run during init
- `DefaultConfig[T](path)` - Generate default config file with random credentials
- `WatchConfig(onChange...)` - Watch config file for changes and auto-reload
- `GetSecureCopy()` / `LogConfig()` - Masked config for safe logging

**Internal Viper fork** (`internal/viper/`): Full internalized copy of spf13/viper with encoding support for YAML, JSON, TOML, and dotenv. This is not a thin wrapper - it's the complete Viper codebase maintained in-tree.

**Config struct flow**: YAML/JSON file -> Viper reads/unmarshals -> `Config` struct populated (with `mapstructure`/`yaml`/`json` tags) -> `defaultValues()` validates and fills defaults -> profile overlay merged -> custom validators run.

## Key Patterns

- Concurrency: `sync.RWMutex` protects `globalConfig`. Write-lock on init/set functions, read-lock on get functions
- Validation in `defaultValues()`: AppID >= 8 chars, AppSecret >= 12 chars, LogLevel must be DEBUG/INFO/WARN/ERROR
- Custom validators: `AddValidator(func(Config) error)` — runs after built-in validation during `InitServiceConfig`
- Config profiles: `config.{environment}.yaml` auto-merged on top of base config (e.g., `config.prod.yaml`)
- Config files: YAML or JSON (extension-based detection for both reading and writing). Auto-creates default file if missing
- Atomic writes: `writeToFile` writes to temp file then renames to prevent data loss (encodes as JSON for `.json`, YAML otherwise)
- File watching: `WatchConfig()` uses fsnotify to auto-reload on changes, re-validates, invokes optional callback
- Filesystem abstracted via `spf13/afero` (enables testing)
- Sensitive field masking: `GetSecureCopy()` uses reflection to mask any field tagged `sensitive:"true"` in both base and service config
- Assertions use `stretchr/testify` (assert + require)
- Tests use `resetGlobalConfig(t)` + `t.Cleanup()` for isolation between tests
- Example tests in `example_test.go` serve as living documentation (show up in `go doc`)

## CI

GitHub Actions runs on non-main branches via reusable workflow (`inovacc/workflows/.github/workflows/reusable-go-check.yml`) with tests, lint, and vulncheck.

## Linting

Configured in `.golangci.yml` (v2 format). Uses `default: all` with only a `disable` block. Notable: depguard allowlist restricts which packages can be imported.
