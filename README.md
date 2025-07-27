[![CI/CD](https://github.com/dyammarcano/config/actions/workflows/ci.yml/badge.svg)](https://github.com/dyammarcano/config/actions/workflows/ci.yml)

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

	config.SetServiceConfig(svc, "config.yaml")

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
├── core/
│   ├── config.go          # Config base, InitConfig, registro de extensiones
│   ├── interface.go       # interface ConfigExtension
│   └── default.go         # DefaultConfig, logging, escritura
├── protobuf/
│   └── extension.go       # Soporte para pb.Config como extensión
│   └── marshal.go         # Export/ImportEncoded
└── go.mod

```