package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/config/internal/viper"
)

func setupBenchmarkConfig(b *testing.B) {
	b.Helper()

	mu.Lock()
	globalConfig = &Config{
		Logger: Logger{LogLevel: "DEBUG"},
	}
	globalConfig.viper = viper.New()
	mu.Unlock()

	dir := b.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
appID: benchmark-app-id-12345
appSecret: benchmark-secret-123456789
logger:
  logLevel: INFO
service:
  username: bench-user
  password: bench-pass
`), 0644)

	if err := InitServiceConfig(&customService{}, cfgPath); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkGetServiceConfig(b *testing.B) {
	setupBenchmarkConfig(b)

	b.ResetTimer()
	for range b.N {
		_, _ = GetServiceConfig[*customService]()
	}
}

func BenchmarkGetBaseConfig(b *testing.B) {
	setupBenchmarkConfig(b)

	b.ResetTimer()
	for range b.N {
		_ = GetBaseConfig()
	}
}

func BenchmarkGetSecureCopy(b *testing.B) {
	setupBenchmarkConfig(b)

	b.ResetTimer()
	for range b.N {
		_ = GetSecureCopy()
	}
}

func BenchmarkMaskSensitiveFields(b *testing.B) {
	svc := &customService{
		Username: "admin",
		Password: "secret123",
	}

	b.ResetTimer()
	for range b.N {
		_ = maskSensitiveFields(svc)
	}
}

func BenchmarkInitServiceConfig(b *testing.B) {
	dir := b.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(`
appID: benchmark-app-id-12345
appSecret: benchmark-secret-123456789
logger:
  logLevel: INFO
service:
  username: bench-user
  password: bench-pass
`), 0644)

	b.ResetTimer()
	for range b.N {
		mu.Lock()
		globalConfig = &Config{
			Logger: Logger{LogLevel: "DEBUG"},
		}
		globalConfig.viper = viper.New()
		mu.Unlock()

		_ = InitServiceConfig(&customService{}, cfgPath)
	}
}

func BenchmarkEncryptDecrypt(b *testing.B) {
	mu.Lock()
	globalConfig = &Config{
		Logger: Logger{LogLevel: "DEBUG"},
	}
	globalConfig.viper = viper.New()
	mu.Unlock()

	SetEncryptionKey([]byte("benchmark-key"))

	b.ResetTimer()
	for range b.N {
		enc, _ := EncryptValue("benchmark-secret-value")
		_, _ = DecryptValue(enc)
	}
}

func BenchmarkEncryptValue(b *testing.B) {
	mu.Lock()
	globalConfig = &Config{
		Logger: Logger{LogLevel: "DEBUG"},
	}
	globalConfig.viper = viper.New()
	mu.Unlock()

	SetEncryptionKey([]byte("benchmark-key"))

	b.ResetTimer()
	for range b.N {
		_, _ = EncryptValue("benchmark-secret-value")
	}
}

func BenchmarkDecryptValue(b *testing.B) {
	mu.Lock()
	globalConfig = &Config{
		Logger: Logger{LogLevel: "DEBUG"},
	}
	globalConfig.viper = viper.New()
	mu.Unlock()

	SetEncryptionKey([]byte("benchmark-key"))
	enc, _ := EncryptValue("benchmark-secret-value")

	b.ResetTimer()
	for range b.N {
		_, _ = DecryptValue(enc)
	}
}
