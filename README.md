[![Test](https://github.com/inovacc/config/actions/workflows/test.yml/badge.svg)](https://github.com/inovacc/config/actions/workflows/test.yml)

# Config Module

A flexible configuration management module for Go applications with support for YAML/JSON files, environment variables,
and type-safe access to configuration values.

## Installation

```shell
go get github.com/inovacc/config
```

## Features

- Load configuration from YAML or JSON files
- Type-safe access to service-specific configuration using generics
- Support for environment variable overrides with custom prefixes
- Secure handling of sensitive configuration values
- Automatic generation of default configuration files with sensible defaults
- Built-in validation for configuration values
- Structured logging integration
- Based on a customized version of Viper for configuration management

## Quick Start

### Loading and Using Configuration

```go
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/config"
)

type ServiceConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

func main() {
	// Initialize with default values
	svc := &ServiceConfig{
		Port: 8080,
		Host: "localhost",
	}

	// Load configuration from a file, applying defaults if needed
	if err := config.InitServiceConfig(svc, "config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get the loaded configuration with type safety
	cfg, err := config.GetServiceConfig[*ServiceConfig]()
	if err != nil {
		log.Fatalf("Failed to get service config: %v", err)
	}

	fmt.Printf("Service running on %s:%d\n", cfg.Host, cfg.Port)

	// Access base configuration
	baseCfg := config.GetBaseConfig()
	fmt.Printf("Application ID: %s\n", baseCfg.AppID)
}
```

## Advanced Features

### Validation Rules and Default Values

The config module includes built-in validation for configuration values:

- `AppID`: Must be at least 8 characters long. If not provided, a UUID is automatically generated.
- `AppSecret`: Must be at least 12 characters long. If not provided, a UUID is automatically generated.
- `Logger.LogLevel`: Must be one of "DEBUG", "INFO", "WARN", "WARNING", or "ERROR" (case-insensitive).

### Creating Default Configuration

You can generate a default configuration file with random credentials using the `DefaultConfig` function:

```go
// Generate a default config file with a zeroed MyServiceConfig
if err := config.DefaultConfig[*MyServiceConfig]("config.yaml"); err != nil {
    log.Fatal(err)
}
```

### Environment Variable Overrides

You can override configuration values using environment variables:

```go
// Set a prefix for environment variables
config.SetEnvPrefix("APP")

// Now environment variables like APP_PORT will override config values
// For example, setting APP_LOGGER_LOGLEVEL=INFO will override logger.logLevel
```

### Secure Handling of Sensitive Values

Mark any string field with `sensitive:"true"` and it will be automatically masked in secure copies. This works for both
the base `AppSecret` field and any fields in your service configuration struct:

```go
type MyConfig struct {
    Username string `yaml:"username"`
    Password string `yaml:"password" sensitive:"true"`
}

// Get a copy with sensitive values masked (AppSecret + Password both masked)
secureCfg := config.GetSecureCopy()

// Log configuration safely
config.LogConfig()
```

### Custom Validation Rules

Register custom validators that run during `InitServiceConfig` after built-in validation:

```go
config.AddValidator(func(cfg config.Config) error {
    svc, ok := cfg.Service.(*MyServiceConfig)
    if !ok {
        return fmt.Errorf("unexpected service config type")
    }
    if svc.Port < 1024 || svc.Port > 65535 {
        return fmt.Errorf("port must be between 1024 and 65535, got %d", svc.Port)
    }
    return nil
})
```

### Configuration Profiles

Profile-specific config files are automatically merged on top of the base config. The profile is determined by the
`environment` field. For example, if `environment: prod` and the base config is `config.yaml`, the library looks for
`config.prod.yaml` in the same directory:

```yaml
# config.yaml (base)
environment: prod
logger:
  logLevel: DEBUG

# config.prod.yaml (profile override — optional)
logger:
  logLevel: ERROR
```

The profile file is optional — if it doesn't exist, the base config is used as-is.

### Configuration Reloading

Watch the config file for changes and automatically reload:

```go
config.WatchConfig(func() {
    log.Println("config reloaded")
})
```

On each file change, the library re-reads the config, merges any profile overrides, and runs custom validators.
If validation fails, the change is rejected and the previous valid config is preserved.

### Configuration Encryption

Encrypt sensitive values at rest using AES-256-GCM. Encrypted values are stored as `ENC[base64data]` in config files and transparently decrypted during loading:

```go
// Set the encryption key (from env var, vault, etc.)
config.SetEncryptionKey([]byte(os.Getenv("CONFIG_KEY")))

// Encrypt a value for storage in a config file
encrypted, err := config.EncryptValue("my-database-password")
// encrypted = "ENC[base64...]"

// In your config.yaml:
// service:
//   password: ENC[base64...]

// Values are automatically decrypted during InitServiceConfig
```

### Configuration Versioning & Migration

Support for versioning config files and migrating between schema versions:

```go
// Set the expected config version
config.SetTargetVersion(3)

// Register migrations (run in order during InitServiceConfig)
config.AddMigration(1, 2, func(data map[string]any) error {
    // Transform config data from v1 to v2
    data["version"] = 2
    if logger, ok := data["logger"].(map[string]any); ok {
        logger["format"] = "json" // Add new field
    }
    return nil
})

config.AddMigration(2, 3, func(data map[string]any) error {
    data["version"] = 3
    return nil
})
```

Add a `version` field to your config file:

```yaml
version: 1
appID: my-app-id-12345678
# ...
```

## Project Structure

```text
github.com/inovacc/config/
├── config.go          # Main implementation (init, get, validate, profiles, watch)
├── encrypt.go         # AES-256-GCM encryption/decryption for config values
├── migrate.go         # Configuration versioning and migration chain
├── config_test.go     # Core tests
├── encrypt_test.go    # Encryption tests
├── migrate_test.go    # Migration tests
├── benchmark_test.go  # Performance benchmarks
├── example_test.go    # Example tests (living documentation)
├── go.mod             # Module definition
├── internal/          # Internal packages
│   └── viper/         # Customized version of Viper
├── IMPROVEMENTS.md    # Completed improvements log
├── Taskfile.yml       # Task runner configuration
└── testdata/          # Test data (YAML and JSON samples)
```

## Configuration Example

Below is an example of a `config.yml` file based on the module's structure:

```yaml
appversion: 0.0.0-development
environment: dev
appID: 3222706d-aa89-4737-a6e3-46d29a7b8b02
appSecret: a6e780be-8b0b-4f5d-b907-72ae0d651eb8
logger:
  logLevel: DEBUG
service:
  username: ""
  password: ""
```

## Improvements

All planned improvements have been implemented. See [IMPROVEMENTS.md](IMPROVEMENTS.md) for the full list of completed features.

## Acknowledgments

This project is based on a customized version of [Viper](https://github.com/spf13/viper), originally created by Steve Francia ([@spf13](https://github.com/spf13)). We would like to express our gratitude to Steve and all the contributors to the Viper project for their excellent work.
