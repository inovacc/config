package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type customService struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

const testFile = "./testdata/config.yaml"

func TestSetServiceConfig(t *testing.T) {
	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)

	require.Equal(t, "tuser", cfg.Username)
}
