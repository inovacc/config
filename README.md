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

The base configuration includes built-in masking for the `AppSecret` field. You can mark additional fields as sensitive
with the `sensitive:"true"` tag (though note that currently only the base `AppSecret` field is automatically masked):

```go
type MyConfig struct {
    Username string `yaml:"username"`
    Password string `yaml:"password" sensitive:"true"`
}

// Get a copy with sensitive values masked
secureCfg := config.GetSecureCopy()

// Log configuration safely
config.LogConfig()
```

> **Note**: Currently, only the `AppSecret` field in the base configuration is automatically masked. Support for
> automatically masking custom fields marked with `sensitive:"true"` is planned for a future release. See
> IMPROVEMENTS.md
> for more details.

## Project Structure

```text
github.com/inovacc/config/
├── config.go         # Main implementation
├── config_test.go    # Tests
├── go.mod            # Module definition
├── go.sum            # Dependencies
├── internal/         # Internal packages
│   └── viper/        # Customized version of Viper
├── LICENSE           # License information
├── README.md         # Documentation
├── IMPROVEMENTS.md   # Improvement suggestions and future plans
├── Taskfile.yml      # Task runner configuration
└── testdata/         # Test data
    └── config.yaml   # Sample configuration
```

## Configuration Example

Below is an example of a `config.yml` file based on the module's structure:

```yaml
# Base configuration
appID: 7c75eaa3-81f4-434f-b774-5208270525cd
appSecret: 27fb1f22-6242-4e69-b04b-0355b87e8ee4

# Logger configuration
logger:
  logLevel: INFO  # Can be DEBUG, INFO, WARN, WARNING, or ERROR

# Service-specific configuration
service:
  # Example for a web service
  host: localhost
  port: 8080
  timeout: 30s

  # Database configuration example
  database:
    host: db.example.com
    port: 5432
    username: dbuser
    password: dbpass  # sensitive value
    name: myapp
    maxConnections: 10

  # Feature flags example
  features:
    enableCache: true
    enableMetrics: true

  # API configuration example
  api:
    rateLimitPerMinute: 100
    authRequired: true
```

## Future Improvements

The module has several planned improvements documented in the IMPROVEMENTS.md file, including:

- Configuration reloading: Support for watching configuration files for changes
- Reflection-based sensitive value handling: Enhance `GetSecureCopy` to mask all fields with the `sensitive:"true"` tag
- Custom validation rules: Support for custom validation of configuration values
- Configuration versioning: Support for versioning and migration
- Configuration encryption: Support for encrypting sensitive values
- Configuration profiles: Support for different environments (dev, test, prod)
- Comprehensive testing: More tests for edge cases

For more details, see the [IMPROVEMENTS.md](IMPROVEMENTS.md) file.

## Acknowledgments

This project is based on a customized version of [Viper](https://github.com/spf13/viper), originally created by Steve Francia ([@spf13](https://github.com/spf13)). We would like to express our gratitude to Steve and all the contributors to the Viper project for their excellent work.
