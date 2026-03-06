package config_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/config"
)

type ExampleServiceConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type ExampleSecureConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password" sensitive:"true"`
}

func Example() {
	dir, _ := os.MkdirTemp("", "example-*")
	defer func() { _ = os.RemoveAll(dir) }()

	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
appID: example-app-id-12345
appSecret: example-secret-12345678
logger:
  logLevel: INFO
service:
  port: 8080
  host: localhost
`), 0644)

	svc := &ExampleServiceConfig{}

	if err := config.InitServiceConfig(svc, cfgPath); err != nil {
		fmt.Println("error:", err)
		return
	}

	cfg, err := config.GetServiceConfig[*ExampleServiceConfig]()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	base := config.GetBaseConfig()
	fmt.Printf("AppID: %s\n", base.AppID)
	fmt.Printf("Host: %s, Port: %d\n", cfg.Host, cfg.Port)

	// Output:
	// AppID: example-app-id-12345
	// Host: localhost, Port: 8080
}

func ExampleGetSecureCopy() {
	dir, _ := os.MkdirTemp("", "example-secure-*")
	defer func() { _ = os.RemoveAll(dir) }()

	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
appID: example-app-id-12345
appSecret: example-secret-12345678
logger:
  logLevel: INFO
service:
  username: admin
  password: s3cret!
`), 0644)

	svc := &ExampleSecureConfig{}

	if err := config.InitServiceConfig(svc, cfgPath); err != nil {
		fmt.Println("error:", err)
		return
	}

	secure := config.GetSecureCopy()
	fmt.Printf("AppSecret: %s\n", secure.AppSecret)

	svcCopy, ok := secure.Service.(*ExampleSecureConfig)
	if ok {
		fmt.Printf("Username: %s\n", svcCopy.Username)
		fmt.Printf("Password: %s\n", svcCopy.Password)
	}

	// Output:
	// AppSecret: ********
	// Username: admin
	// Password: ********
}

func ExampleAddValidator() {
	dir, _ := os.MkdirTemp("", "example-validator-*")
	defer func() { _ = os.RemoveAll(dir) }()

	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
appID: example-app-id-12345
appSecret: example-secret-12345678
logger:
  logLevel: INFO
service:
  port: 80
  host: localhost
`), 0644)

	config.AddValidator(func(cfg config.Config) error {
		svc, ok := cfg.Service.(*ExampleServiceConfig)
		if !ok {
			return fmt.Errorf("unexpected service type")
		}
		if svc.Port < 1024 {
			return fmt.Errorf("port must be >= 1024, got %d", svc.Port)
		}
		return nil
	})

	err := config.InitServiceConfig(&ExampleServiceConfig{}, cfgPath)
	fmt.Println(err)

	// Output:
	// custom validation: port must be >= 1024, got 80
}
