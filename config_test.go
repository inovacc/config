package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	obj := struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	}{
		Username: "jhon",
		Password: "admin",
	}

	if err := DefaultConfig(obj, "."); err != nil {
		t.Errorf("Error creating default config: %v", err)
	}
}
