package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type customService struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func TestDefaultConfig(t *testing.T) {
	err := DefaultConfig[customService]("config.yaml")
	require.NoError(t, err)
}

func TestSetServiceConfig(t *testing.T) {
	err := SetServiceConfig(&customService{}, "config.yaml")
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)

	require.Equal(t, "myuser", cfg.Username)
}
