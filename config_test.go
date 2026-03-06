package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/inovacc/config/internal/viper"
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

// resetGlobalConfig resets the global config to a clean initial state.
func resetGlobalConfig(t *testing.T) {
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()

		globalConfig = &Config{
			Logger: Logger{
				LogLevel: "DEBUG",
			},
		}
		globalConfig.viper = viper.New()
	})
}

// setupTestDir creates a temporary directory for testing
func setupTestDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "config-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	return tempDir
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
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)

	require.Equal(t, "tuser", cfg.Username)
}

// TestInitServiceConfigNonExistingFile tests InitServiceConfig with a non-existing file
func TestInitServiceConfigNonExistingFile(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

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
	resetGlobalConfig(t)

	svc := &customService{}
	err := InitServiceConfig(svc, "/invalid/path/that/should/not/exist/config.yaml")
	assert.Error(t, err)
}

// TestDefaultConfig tests the DefaultConfig function
func TestDefaultConfig(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

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
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	// Try to get the service config with the wrong type
	_, err = GetServiceConfig[*anotherService]()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid service config type")
}

// TestValidationRules1 tests the validation rules for AppID, AppSecret, and LogLevel
func TestValidationRules1(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

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

// TestValidationRules2 tests validation rules using table-driven tests
func TestValidationRules2(t *testing.T) {
	tempDir := setupTestDir(t)

	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			"invalid AppID",
			`appID: short
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass`,
			"invalid AppID",
		},
		{
			"invalid AppSecret",
			`appID: validappid12345
appSecret: short
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass`,
			"invalid AppSecret",
		},
		{
			"invalid LogLevel",
			`appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: INVALID
service:
  username: testuser
  password: testpass`,
			"unknown log level",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetGlobalConfig(t)

			configPath := createTestConfig(t, tempDir, tc.name+".yaml", tc.content)
			err := InitServiceConfig(&customService{}, configPath)
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TestGetSecureCopy tests the GetSecureCopy function
func TestGetSecureCopy(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	// Get the original config
	cfg := GetBaseConfig()
	assert.NotEmpty(t, cfg.AppSecret)

	// Get a secure copy
	secureCfg := GetSecureCopy()
	assert.Equal(t, "********", secureCfg.AppSecret)

	// Verify original is NOT modified (GetBaseConfig returns a copy)
	cfg2 := GetBaseConfig()
	assert.NotEqual(t, "********", cfg2.AppSecret)
}

// TestGetSecureCopyMasksServiceFields tests that GetSecureCopy masks sensitive
// fields in the service config struct using reflection.
func TestGetSecureCopyMasksServiceFields(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	// Get original service config
	svc, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.NotEmpty(t, svc.Password)
	originalPassword := svc.Password

	// Get a secure copy
	secureCfg := GetSecureCopy()

	// Verify the service password is masked in the secure copy
	maskedSvc, ok := secureCfg.Service.(*customService)
	require.True(t, ok)
	assert.Equal(t, "********", maskedSvc.Password)
	assert.Equal(t, svc.Username, maskedSvc.Username)

	// Verify original service config is NOT modified
	assert.Equal(t, originalPassword, svc.Password)
}

// TestGetSecureCopyNilService tests GetSecureCopy when service is nil
func TestGetSecureCopyNilService(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configPath := filepath.Join(tempDir, "config.yaml")

	err := DefaultConfig[*anotherService](configPath)
	require.NoError(t, err)

	secureCfg := GetSecureCopy()
	assert.Equal(t, "********", secureCfg.AppSecret)
}

// TestEnvironmentVariables tests environment variable overrides
func TestEnvironmentVariables(t *testing.T) {
	resetGlobalConfig(t)

	// Set environment variables
	_ = os.Setenv("TEST_LOGGER_LOGLEVEL", "INFO")
	t.Cleanup(func() {
		_ = os.Unsetenv("TEST_LOGGER_LOGLEVEL")
		SetEnvPrefix("")
	})

	tempDir := setupTestDir(t)

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
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

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

// TestMultipleInitServiceConfig tests calling InitServiceConfig multiple times
func TestMultipleInitServiceConfig(t *testing.T) {
	resetGlobalConfig(t)

	// First init with customService
	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	cfg1, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "tuser", cfg1.Username)

	// Second init with different type overwrites
	tempDir := setupTestDir(t)

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  port: 9090
  host: example.com
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	err = InitServiceConfig(&anotherService{}, configPath)
	require.NoError(t, err)

	cfg2, err := GetServiceConfig[*anotherService]()
	require.NoError(t, err)
	assert.Equal(t, "example.com", cfg2.Host)

	// Old type should now fail
	_, err = GetServiceConfig[*customService]()
	assert.Error(t, err)
}

// TestLogConfig tests the LogConfig function does not panic
func TestLogConfig(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		LogConfig()
	})
}

// TestMaskSensitiveFields tests the reflection-based sensitive field masking
func TestMaskSensitiveFields(t *testing.T) {
	// Test with struct pointer
	svc := &customService{
		Username: "admin",
		Password: "secret123",
	}
	masked := maskSensitiveFields(svc)
	maskedSvc, ok := masked.(*customService)
	require.True(t, ok)
	assert.Equal(t, "admin", maskedSvc.Username)
	assert.Equal(t, "********", maskedSvc.Password)

	// Verify original is not modified
	assert.Equal(t, "secret123", svc.Password)

	// Test with nil
	assert.Nil(t, maskSensitiveFields(nil))

	// Test with non-struct (should return unchanged)
	s := "hello"
	assert.Equal(t, "hello", maskSensitiveFields(s))

	// Test with struct that has no sensitive fields
	svc2 := &anotherService{Port: 8080, Host: "localhost"}
	masked2 := maskSensitiveFields(svc2)
	maskedSvc2, ok := masked2.(*anotherService)
	require.True(t, ok)
	assert.Equal(t, 8080, maskedSvc2.Port)
	assert.Equal(t, "localhost", maskedSvc2.Host)

	// Test with empty sensitive field (should not mask)
	svc3 := &customService{Username: "admin", Password: ""}
	masked3 := maskSensitiveFields(svc3)
	maskedSvc3, ok := masked3.(*customService)
	require.True(t, ok)
	assert.Equal(t, "", maskedSvc3.Password)
}

// TestGetBaseConfigReturnsCopy tests that GetBaseConfig returns a copy, not a pointer
func TestGetBaseConfigReturnsCopy(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	cfg := GetBaseConfig()
	assert.NotNil(t, cfg.AppID)

	// Modifying the returned copy should NOT affect the global config
	originalAppID := cfg.AppID
	cfg.AppID = "mutated-value"

	cfg2 := GetBaseConfig()
	assert.Equal(t, originalAppID, cfg2.AppID)
}

// TestInitServiceConfigErrorPropagation tests that errors from defaultConfig are
// properly propagated (not swallowed)
func TestInitServiceConfigErrorPropagation(t *testing.T) {
	resetGlobalConfig(t)

	// Use a path where directory doesn't exist, so file creation fails
	err := InitServiceConfig(&customService{}, "/nonexistent/dir/config.yaml")
	assert.Error(t, err)
	// Should NOT be the old generic message
	assert.NotContains(t, err.Error(), "configuration file not found, please verify")
}

// TestAddValidator tests custom validator registration and execution
func TestAddValidator(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  port: 80
  host: localhost
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	// Register a validator that rejects ports below 1024
	AddValidator(func(cfg Config) error {
		svc, ok := cfg.Service.(*anotherService)
		if !ok {
			return fmt.Errorf("unexpected service type")
		}
		if svc.Port < 1024 {
			return fmt.Errorf("port must be >= 1024, got %d", svc.Port)
		}
		return nil
	})

	err := InitServiceConfig(&anotherService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "port must be >= 1024")
		assert.Contains(t, err.Error(), "custom validation")
	}
}

// TestAddValidatorPasses tests that validators that pass don't block init
func TestAddValidatorPasses(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  port: 8080
  host: localhost
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	AddValidator(func(cfg Config) error {
		svc, ok := cfg.Service.(*anotherService)
		if !ok {
			return fmt.Errorf("unexpected service type")
		}
		if svc.Port < 1024 {
			return fmt.Errorf("port must be >= 1024, got %d", svc.Port)
		}
		return nil
	})

	err := InitServiceConfig(&anotherService{}, configPath)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*anotherService]()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Port)
}

// TestMultipleValidators tests that multiple validators run in order
func TestMultipleValidators(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: ""
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	// First validator passes
	AddValidator(func(_ Config) error {
		return nil
	})

	// Second validator fails
	AddValidator(func(cfg Config) error {
		svc, ok := cfg.Service.(*customService)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		if svc.Username == "" {
			return fmt.Errorf("username is required")
		}
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "username is required")
	}
}

// TestConfigProfile tests loading profile-specific config overrides
func TestConfigProfile(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	// Base config
	baseContent := `
appID: validappid12345
appSecret: validappsecret12345
environment: prod
logger:
  logLevel: DEBUG
service:
  username: dev-user
  password: dev-pass
`
	createTestConfig(t, tempDir, "config.yaml", baseContent)

	// Profile-specific config (prod)
	prodContent := `
logger:
  logLevel: ERROR
service:
  username: prod-user
`
	createTestConfig(t, tempDir, "config.prod.yaml", prodContent)

	configPath := filepath.Join(tempDir, "config.yaml")
	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	// Verify profile values override base values
	cfg := GetBaseConfig()
	assert.Equal(t, "ERROR", cfg.Logger.LogLevel)

	svc, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "prod-user", svc.Username)
}

// TestConfigProfileNotFound tests that missing profile files are silently skipped
func TestConfigProfileNotFound(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	baseContent := `
appID: validappid12345
appSecret: validappsecret12345
environment: staging
logger:
  logLevel: INFO
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", baseContent)

	// No config.staging.yaml exists — should be fine
	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	cfg := GetBaseConfig()
	assert.Equal(t, "INFO", cfg.Logger.LogLevel)
	assert.Equal(t, "staging", cfg.Environment)
}

// TestConcurrentGetServiceConfig tests concurrent reads are safe
func TestConcurrentGetServiceConfig(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			cfg, err := GetServiceConfig[*customService]()
			assert.NoError(t, err)
			assert.NotEmpty(t, cfg.Username)
		}()
	}

	wg.Wait()
}

// TestConcurrentGetBaseConfig tests concurrent reads of base config are safe
func TestConcurrentGetBaseConfig(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testFile)
	require.NoError(t, err)

	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			cfg := GetBaseConfig()
			assert.NotEmpty(t, cfg.AppID)

			secureCfg := GetSecureCopy()
			assert.Equal(t, "********", secureCfg.AppSecret)
		}()
	}

	wg.Wait()
}

// TestAtomicWriteToFile tests that writeToFile creates the file atomically
func TestAtomicWriteToFile(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configPath := filepath.Join(tempDir, "config.yaml")

	// Generate defaults so globalConfig has valid data
	mu.Lock()
	err := defaultConfig(configPath)
	mu.Unlock()
	require.NoError(t, err)

	// Verify the file was created
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "appID")

	// Verify no temp files are left behind
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".config-"), "temp file left behind: %s", e.Name())
	}
}

// --- JSON config file tests ---

const testJSONFile = "./testdata/config.json"

// TestInitServiceConfigJSON tests loading a JSON config file
func TestInitServiceConfigJSON(t *testing.T) {
	resetGlobalConfig(t)

	err := InitServiceConfig(&customService{}, testJSONFile)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "json-user", cfg.Username)
	assert.Equal(t, "json-pass", cfg.Password)

	base := GetBaseConfig()
	assert.Equal(t, "json-app-id-12345678", base.AppID)
	assert.Equal(t, "INFO", base.Logger.LogLevel)
}

// TestDefaultConfigJSON tests generating a default JSON config file
func TestDefaultConfigJSON(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configPath := filepath.Join(tempDir, "config.json")

	err := DefaultConfig[*anotherService](configPath)
	require.NoError(t, err)

	// Verify the file was created and is valid JSON
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "appID")

	// Init with the generated JSON config
	err = InitServiceConfig(&anotherService{}, configPath)
	require.NoError(t, err)

	cfg := GetBaseConfig()
	assert.NotEmpty(t, cfg.AppID)
	assert.Equal(t, "DEBUG", cfg.Logger.LogLevel)
}

// TestConfigProfileJSON tests profile loading with JSON files
func TestConfigProfileJSON(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	baseContent := `{
  "appID": "json-app-id-12345678",
  "appSecret": "json-secret-123456789012",
  "environment": "staging",
  "logger": {"logLevel": "DEBUG"},
  "service": {"username": "base-user", "password": "base-pass"}
}`
	createTestConfig(t, tempDir, "config.json", baseContent)

	profileContent := `{
  "logger": {"logLevel": "WARN"},
  "service": {"username": "staging-user"}
}`
	createTestConfig(t, tempDir, "config.staging.json", profileContent)

	configPath := filepath.Join(tempDir, "config.json")
	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	cfg := GetBaseConfig()
	assert.Equal(t, "WARN", cfg.Logger.LogLevel)

	svc, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "staging-user", svc.Username)
}

// --- WatchConfig test ---

// TestWatchConfig tests that WatchConfig detects file changes
func TestWatchConfig(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: original
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	cfg, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "original", cfg.Username)

	// Start watching
	reloaded := make(chan struct{}, 1)
	WatchConfig(func() {
		select {
		case reloaded <- struct{}{}:
		default:
		}
	})

	// Modify the config file
	updatedContent := `
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: INFO
service:
  username: updated
  password: testpass
`
	err = os.WriteFile(configPath, []byte(updatedContent), 0644)
	require.NoError(t, err)

	// Wait for reload callback
	select {
	case <-reloaded:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for config reload")
	}

	cfg, err = GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "updated", cfg.Username)

	base := GetBaseConfig()
	assert.Equal(t, "INFO", base.Logger.LogLevel)
}
