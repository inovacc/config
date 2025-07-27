[![CI/CD](https://github.com/dyammarcano/config/actions/workflows/ci.yml/badge.svg)](https://github.com/dyammarcano/config/actions/workflows/ci.yml)

# Preparar un config nuevo

```go
package main

import (
	"log"

	"github.com/dyammarcano/config"
)

type ServiceConfig struct {
	Port int `json:"port" yaml:"port"`
}

func main() {
	if err := config.DefaultConfig[ServiceConfig]("config.yaml"); err != nil {
		log.Fatalf("failed to set config: %v", err)
	}
}

```

# Como usar

```go
package main

import (
	"fmt"
	"log"

	"github.com/dyammarcano/config"
)

type ServiceConfig struct {
	Port int `json:"port" yaml:"port"`
}

func main() {
	svc := &ServiceConfig{
		Port: 8080,
	}

	if err := config.InitServiceConfig(svc, "config.yaml"); err != nil {
		log.Fatalf("failed to set config: %v", err)
	}

	cfg, err := config.GetServiceConfig[*ServiceConfig]()
	if err != nil {
		log.Fatalf("failed to get config: %v", err)
	}

	fmt.Println("Loaded config, port =", cfg.Port)
}
```

# estructura

```text
github.com/dyammarcano/config/
├── config.go
├── config_test.go
├── go.mod
├── go.sum
├── LICENSE
├── README.md
├── Taskfile.yml
└── testdata
    └── config.yaml

```