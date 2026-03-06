package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationV1ToV2(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(2)
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		// Simulate a migration: add environment field
		if _, ok := data["environment"]; !ok {
			data["environment"] = "dev"
		}
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 2, GetConfigVersion())
	cfg := GetBaseConfig()
	assert.Equal(t, "dev", cfg.Environment)
}

func TestMigrationChain(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(3)

	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		return nil
	})

	AddMigration(2, 3, func(data map[string]any) error {
		data["version"] = 3
		data["environment"] = "staging"
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 3, GetConfigVersion())
	cfg := GetBaseConfig()
	assert.Equal(t, "staging", cfg.Environment)
}

func TestMigrationNoTarget(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	// No SetTargetVersion call — migrations should be skipped
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 1, GetConfigVersion())
}

func TestMigrationAlreadyAtTarget(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 3
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(3)
	migrationCalled := false
	AddMigration(2, 3, func(data map[string]any) error {
		migrationCalled = true
		data["version"] = 3
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.False(t, migrationCalled)
	assert.Equal(t, 3, GetConfigVersion())
}

func TestMigrationError(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(2)
	AddMigration(1, 2, func(_ map[string]any) error {
		return fmt.Errorf("migration failed: incompatible data")
	})

	err := InitServiceConfig(&customService{}, configPath)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "running migrations")
		assert.Contains(t, err.Error(), "migration failed")
	}
}

func TestMigrationModifiesServiceData(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: old-user
  password: old-pass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(2)
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		if svc, ok := data["service"].(map[string]any); ok {
			svc["username"] = "migrated-user"
		}
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	svc, err := GetServiceConfig[*customService]()
	require.NoError(t, err)
	assert.Equal(t, "migrated-user", svc.Username)
}

func TestMigrationNoVersionField(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	// Config without version field — defaults to 0
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

	SetTargetVersion(1)
	AddMigration(0, 1, func(data map[string]any) error {
		data["version"] = 1
		data["environment"] = "migrated"
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 1, GetConfigVersion())
	cfg := GetBaseConfig()
	assert.Equal(t, "migrated", cfg.Environment)
}

func TestMigrationJSON(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `{
  "version": 1,
  "appID": "json-app-id-12345678",
  "appSecret": "json-secret-123456789012",
  "logger": {"logLevel": "INFO"},
  "service": {"username": "json-user", "password": "json-pass"}
}`
	configPath := createTestConfig(t, tempDir, "config.json", configContent)

	SetTargetVersion(2)
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 2, GetConfigVersion())
}

func TestGetConfigVersionBeforeInit(t *testing.T) {
	resetGlobalConfig(t)

	// Before init, version should be 0
	assert.Equal(t, 0, GetConfigVersion())
}

func TestMigrationOutOfOrder(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	configContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
logger:
  logLevel: DEBUG
service:
  username: testuser
  password: testpass
`
	configPath := createTestConfig(t, tempDir, "config.yaml", configContent)

	SetTargetVersion(3)

	// Register out of order — should still work because they're sorted
	AddMigration(2, 3, func(data map[string]any) error {
		data["version"] = 3
		data["environment"] = "final"
		return nil
	})
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		return nil
	})

	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 3, GetConfigVersion())
	cfg := GetBaseConfig()
	assert.Equal(t, "final", cfg.Environment)
}

func TestMigrationWithProfileOverride(t *testing.T) {
	resetGlobalConfig(t)
	tempDir := setupTestDir(t)

	baseContent := `
version: 1
appID: validappid12345
appSecret: validappsecret12345
environment: prod
logger:
  logLevel: DEBUG
service:
  username: base-user
  password: base-pass
`
	createTestConfig(t, tempDir, "config.yaml", baseContent)

	prodContent := `
logger:
  logLevel: ERROR
`
	createTestConfig(t, tempDir, "config.prod.yaml", prodContent)

	SetTargetVersion(2)
	AddMigration(1, 2, func(data map[string]any) error {
		data["version"] = 2
		return nil
	})

	configPath := filepath.Join(tempDir, "config.yaml")
	err := InitServiceConfig(&customService{}, configPath)
	require.NoError(t, err)

	assert.Equal(t, 2, GetConfigVersion())
	cfg := GetBaseConfig()
	assert.Equal(t, "ERROR", cfg.Logger.LogLevel) // profile override still applies
}
