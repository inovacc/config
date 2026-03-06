# Config Module Improvements

This document tracks improvements made to the config module and planned future work.

## Completed Improvements

### 1. Error Handling and Validation

- Added validation for AppID (>= 8 chars) and AppSecret (>= 12 chars) length
- Made log level case-insensitive with support for common names ("WARN", "WARNING")
- Improved error messages with details and suggestions
- Fixed error propagation in `InitServiceConfig` — real errors from `defaultConfig` are now wrapped instead of swallowed

### 2. Environment Variable Prefixes

- Added `SetEnvPrefix` to bind environment variables with a prefix (e.g., `APP_LOGGER_LOGLEVEL`)
- Key replacer converts dots and hyphens to underscores in env var names

### 3. Sensitive Value Handling (Reflection-Based)

- `GetSecureCopy` uses reflection to mask all fields tagged `sensitive:"true"` in both the base `Config` and the service config struct
- `LogConfig` safely logs configuration without exposing sensitive values
- Masking creates copies — originals are never mutated

### 4. Concurrency Safety

- Added `sync.RWMutex` protecting all access to `globalConfig`
- Write-lock on `InitServiceConfig`, `SetEnvPrefix`, `DefaultConfig`, `AddValidator`
- Read-lock on `GetServiceConfig`, `GetBaseConfig`, `GetSecureCopy`, `LogConfig`
- `GetBaseConfig` returns a value copy instead of a mutable pointer

### 5. Atomic File Writes

- `writeToFile` now writes to a temporary file first, then renames to the target path
- Prevents data loss if encoding fails or the process crashes mid-write
- Supports both YAML and JSON output based on file extension

### 6. Custom Validation Rules

- Added `AddValidator(func(Config) error)` to register custom validation functions
- Validators run during `InitServiceConfig` after built-in validation completes
- Multiple validators can be registered and they run in order

### 7. Configuration Profiles

- Profile-specific config files (e.g., `config.prod.yaml`) are automatically merged on top of the base config
- Profile is determined by the `Environment` field value
- Profile file is optional — if it doesn't exist, base config is used as-is

### 8. Configuration Reloading

- `WatchConfig()` watches the config file for changes using fsnotify
- On change: re-reads config, reloads profile, runs validators, invokes optional callback
- Invalid config changes are logged and rejected (previous valid config is preserved)

### 9. JSON Support

- Config struct has both `yaml` and `json` struct tags
- `writeToFile` encodes as JSON when file extension is `.json`, YAML otherwise
- Full test coverage for JSON: init, default generation, profile merging

### 10. Test Suite

- Comprehensive table-driven tests with proper global state isolation via `resetGlobalConfig(t)`
- Tests for: validation rules, secure copy, env var overrides, error handling, type mismatches, multiple init calls, reflection-based masking, custom validators, profile loading, JSON configs, file watching, concurrency
- Example tests (`example_test.go`) serve as living documentation
- Removed library-level `slog.SetDefault()` calls — the library no longer hijacks the application's global logger

### 11. Linter Configuration

- Cleaned up `.golangci.yml` — removed contradictory enable/disable entries
- Uses `default: all` with only a `disable` block

## Future Improvements

1. **Configuration Versioning**: Support for versioning configuration files and migrating between versions.

2. **Configuration Encryption**: Support for encrypting sensitive configuration values at rest.
