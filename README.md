[![Test](https://github.com/dyammarcano/config/actions/workflows/test.yml/badge.svg)](https://github.com/dyammarcano/config/actions/workflows/test.yml)

# Config Module

A flexible configuration management module for Go applications with support for YAML/JSON files, environment variables,
and type-safe access to configuration values.

## Installation

```shell
go get github.com/dyammarcano/config
```

## Features

- Load configuration from YAML or JSON files
- Type-safe access to service-specific configuration using generics
- Support for environment variable overrides with custom prefixes
- Secure handling of sensitive configuration values
- Automatic generation of default configuration files
- Structured logging integration

## Quick Start

### Creating a Default Configuration

```go
package main

import (
	"log"

	"github.com/dyammarcano/config"
)

type ServiceConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

func main() {
	// Generate a default config.yaml file with random AppID and AppSecret
	if err := config.DefaultConfig[ServiceConfig]("config.yaml"); err != nil {
		log.Fatalf("Failed to create default config: %v", err)
	}
}
```

### Loading and Using Configuration

```go
package main

import (
	"fmt"
	"log"

	"github.com/dyammarcano/config"
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

	// Load configuration from file, applying defaults if needed
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

### Environment Variable Overrides

You can override configuration values using environment variables:

```go
// Set a prefix for environment variables
config.SetEnvPrefix("APP")

// Now environment variables like APP_PORT will override config values
// For example, setting APP_LOGGER_LOGLEVEL=INFO will override logger.logLevel
```

### Secure Handling of Sensitive Values

Mark sensitive fields with the `sensitive:"true"` tag:

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

## Project Structure

```text
github.com/dyammarcano/config/
├── config.go         # Main implementation
├── config_test.go    # Tests
├── go.mod            # Module definition
├── go.sum            # Dependencies
├── LICENSE           # License information
├── README.md         # Documentation
├── Taskfile.yml      # Task runner configuration
└── testdata/         # Test data
    └── config.yaml   # Sample configuration
```
