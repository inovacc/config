package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	if err := DefaultConfig("."); err != nil {
		t.Errorf("Error creating default config: %v", err)
	}
}
