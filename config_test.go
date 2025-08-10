package config

// This test suite provides comprehensive coverage of the config package functionality.
// It includes tests for both core features and advanced functionality, as well as
// error handling and edge cases.
//
// Test Organization:
// - Core functionality tests: TestSetServiceConfig, TestInitServiceConfigNonExistingFile,
//   TestInitServiceConfigInvalidPath, TestDefaultConfig, TestGetServiceConfigTypeMismatch
// - Validation tests: TestValidationRules
// - Advanced feature tests: TestGetSecureCopy, TestEnvironmentVariables
// - Error handling tests: TestErrorHandling
//
// Test Patterns:
// - Each test focuses on a specific feature or scenario
// - Helper functions (setupTestDir, createTestConfig) are used to reduce duplication
// - Temporary directories are used to isolate tests and clean up automatically
// - Both positive (success) and negative (error) cases are tested
// - Edge cases are explicitly tested (e.g., invalid paths, type mismatches)
//
// To run the tests: go test -v

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type customService struct {
	Username string `yaml:"username"`
	Password string `yaml:"password" sensitive:"true"`
}

type anotherService struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

const testFile = "./testdata/config.yaml"

// setupTestDir creates a temporary directory for testing
func setupTestDir(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createTestConfig creates a test configuration file
func createTestConfig(t *testing.T, dir string, filename string, content string) string {
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}

// TestSetServiceConfig tests the basic functionality of InitServiceConfig and GetServiceConfig
func TestSetServiceConfig(t *testing.T) {
	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)

	require.Equal(t, "tuser", cfg.Username)
}

// TestInitServiceConfigNonExistingFile tests InitServiceConfig with a non-existing file
func TestInitServiceConfigNonExistingFile(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	configPath := filepath.Join(tempDir, "config.yaml")

	svc := &customService{}
	err := InitServiceConfig(svc, configPath)
	require.NoError(t, err)

	// Verify the file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify the config was loaded correctly
	cfg := GetBaseConfig()
	assert.NotEmpty(t, cfg.AppID)
	assert.NotEmpty(t, cfg.AppSecret)
	assert.Equal(t, "DEBUG", cfg.Logger.LogLevel)
}

// TestInitServiceConfigInvalidPath tests InitServiceConfig with an invalid path
func TestInitServiceConfigInvalidPath(t *testing.T) {
	svc := &customService{}
	err := InitServiceConfig(svc, "/invalid/path/that/should/not/exist/config.yaml")
	assert.Error(t, err)
}

// TestDefaultConfig tests the DefaultConfig function
func TestDefaultConfig(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	configPath := filepath.Join(tempDir, "config.yaml")

	err := DefaultConfig[*anotherService](configPath)
	require.NoError(t, err)

	// Verify the file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Initialize with the created config
	svc := &anotherService{}
	err = InitServiceConfig(svc, configPath)
	require.NoError(t, err)

	// Verify the config was loaded correctly
	cfg := GetBaseConfig()
	assert.NotEmpty(t, cfg.AppID)
	assert.NotEmpty(t, cfg.AppSecret)
	assert.Equal(t, "DEBUG", cfg.Logger.LogLevel)
}

// TestGetServiceConfigTypeMismatch tests GetServiceConfig with a type mismatch
func TestGetServiceConfigTypeMismatch(t *testing.T) {
	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	// Try to get the service config with the wrong type
	_, err = GetServiceConfig[*anotherService]()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid service config type")
}

// TestValidationRules tests the validation rules for AppID, AppSecret, and LogLevel
func TestValidationRules1(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Test invalid AppID
	configContent := `
appID: short
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "invalid_appid.yaml", configContent)

	err := InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid AppID")
	}

	// Test invalid AppSecret
	configContent = `
appID: validappid12345
appSecret: short
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath = createTestConfig(t, tempDir, "invalid_appsecret.yaml", configContent)

	err = InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid AppSecret")
	}

	// Test invalid LogLevel
	configContent = `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: INVALID
service:
  username: testuser
  password: testpass
`
	configPath = createTestConfig(t, tempDir, "invalid_loglevel.yaml", configContent)

	err = InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unknown log level")
	}
}

func TestValidationRules2(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"invalid AppID", "...", "invalid AppID"},
		{"invalid AppSecret", "...", "invalid AppSecret"},
		{"invalid LogLevel", "...", "unknown log level"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := createTestConfig(t, tempDir, "test.yaml", tc.content)
			err := InitServiceConfig(&customService{}, configPath)
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TestGetSecureCopy tests the GetSecureCopy function
func TestGetSecureCopy(t *testing.T) {
	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	// Get the original config
	cfg := GetBaseConfig()
	assert.NotEmpty(t, cfg.AppSecret)

	// Get a secure copy
	secureCfg := GetSecureCopy()
	assert.Equal(t, "********", secureCfg.AppSecret)
}

// TestEnvironmentVariables tests environment variable overrides
func TestEnvironmentVariables(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("TEST_LOGGER_LOGLEVEL", "INFO")
	defer func() {
		_ = os.Unsetenv("TEST_LOGGER_LOGLEVEL")
	}()

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	// Set environment variable prefix
	SetEnvPrefix("TEST")

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	// Verify that the environment variable overrode the config value
	cfg := GetBaseConfig()
	assert.Equal(t, "INFO", cfg.Logger.LogLevel)
}

// TestErrorHandling tests error handling for various scenarios
func TestErrorHandling(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Test invalid YAML
	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
  invalid yaml content
`
	configPath := createTestConfig(t, tempDir, "invalid_yaml.yaml", configContent)

	err := InitServiceConfig(&customService{}, configPath)
	assert.Error(t, err)

	// Test unsupported file extension
	configPath = filepath.Join(tempDir, "config.txt")
	err = os.WriteFile(configPath, []byte("test"), 0644)
	require.NoError(t, err)

	err = InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unsupported config file extension")
	}
}
