package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/inovacc/config/internal/viper"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const maskedValue = "********"

var (
	globalConfig *Config
	mu           sync.RWMutex
)

func init() {
	globalConfig = &Config{
		Logger: Logger{
			LogLevel: slog.LevelDebug.String(),
		},
	}

	globalConfig.viper = viper.New()
}

// Logger defines the configuration for structured logging.
type Logger struct {
	LogLevel string `yaml:"logLevel" json:"logLevel" mapstructure:"logLevel"`
}

// ValidatorFunc is a function that validates the configuration.
// It receives a read-only copy of the Config and should return an error
// if validation fails.
type ValidatorFunc func(Config) error

// Config represents the global application configuration.
//
// Fields:
//   - Environment: The current environment (e.g., "dev", "prod").
//   - AppVersion: The application version.
//   - AppID: Unique application identifier.
//   - AppSecret: Secret key for the application (sensitive).
//   - Logger: Structured logging configuration.
//   - Service: Service-specific configuration.
type Config struct {
	viper       *viper.Viper
	envPrefix   string
	validators  []ValidatorFunc
	Environment string `yaml:"environment" json:"environment" mapstructure:"environment"`
	AppVersion  string `yaml:"-" json:"-" mapstructure:"-"`
	ConfigFile  string `yaml:"-" json:"-" mapstructure:"-"`
	AppID       string `yaml:"appID" json:"appID" mapstructure:"appID"`
	AppSecret   string `yaml:"appSecret" json:"appSecret" mapstructure:"appSecret" sensitive:"true"`
	Logger      Logger `yaml:"logger" json:"logger" mapstructure:"logger"`
	Service     any    `yaml:"service" json:"service" mapstructure:"service"`
}

// InitServiceConfig loads a configuration file and binds a service-specific
// struct to the `Service` field in the global config.
//
// It must be called before accessing the service configuration via GetServiceConfig.
//
// If the configuration file does not exist, a default one will be created.
// Default values from the provided service config struct will be used if
// corresponding values are not found in the configuration file.
//
// After loading, if a profile-specific config file exists (e.g., "config.prod.yaml"
// when Environment is "prod"), its values are merged on top of the base config.
//
// Example:
//
//	type MyServiceConfig struct {
//	    Port int
//	    Mode string
//	}
//
//	svc := &MyServiceConfig{
//	    Port: 8080,  // Default value
//	    Mode: "dev", // Default value
//	}
//
//	err := config.InitServiceConfig(svc, "config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
func InitServiceConfig(v any, configPath string) error {
	mu.Lock()
	defer mu.Unlock()

	afs := afero.NewOsFs()

	configFile, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	globalConfig.ConfigFile = configFile
	globalConfig.Service = v

	// Check if a config file exists, create default if not
	if !exists(afs, configFile) {
		slog.Warn("Configuration file not found, creating default, please verify", "path", configFile)

		if err := defaultConfig(configPath); err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
	}

	// Read configuration from a file
	if err = globalConfig.readInConfig(afs); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	// Set default values
	if err = globalConfig.defaultValues(); err != nil {
		return fmt.Errorf("setting default values: %w", err)
	}

	// Load profile-specific overrides
	if err = globalConfig.loadProfile(afs); err != nil {
		return fmt.Errorf("loading profile config: %w", err)
	}

	// Run custom validators
	if err = globalConfig.runValidators(); err != nil {
		return fmt.Errorf("custom validation: %w", err)
	}

	// Log the configuration (safely masking sensitive values)
	logConfigLocked()

	return nil
}

// GetServiceConfig returns the previously registered service-specific configuration
// with type safety using generics.
//
// If the type does not match what was stored, an error is returned.
//
// Example:
//
//	cfg, err := config.GetServiceConfig[*MyServiceConfig]()
//	if err != nil {
//	    log.Fatal(err)
//	}
func GetServiceConfig[T any]() (T, error) {
	mu.RLock()
	defer mu.RUnlock()

	var zero T
	val, ok := globalConfig.Service.(T)
	if !ok {
		return zero, fmt.Errorf("invalid service config type: expected %T, got %T", zero, globalConfig.Service)
	}
	return val, nil
}

// GetBaseConfig returns a copy of the global configuration base object.
//
// This allows safe read access to common fields like AppID, Logger, and AppSecret
// without exposing the global state to mutation.
//
// Example:
//
//	cfg := config.GetBaseConfig()
//	fmt.Println("AppID:", cfg.AppID)
func GetBaseConfig() Config {
	mu.RLock()
	defer mu.RUnlock()

	return *globalConfig
}

// SetEnvPrefix sets a prefix for environment variables.
//
// Environment variables that match the pattern {prefix}_* will override
// the corresponding configuration values. The matching is case-insensitive.
//
// For example, if the prefix is "APP", then the environment variable "APP_LOGGER_LOGLEVEL"
// will override the value of "logger.logLevel" in the configuration file.
//
// Must be called before InitServiceConfig.
//
// Example:
//
//	config.SetEnvPrefix("APP")
func SetEnvPrefix(prefix string) {
	mu.Lock()
	defer mu.Unlock()

	globalConfig.envPrefix = prefix
}

// AddValidator registers a custom validation function that will be called
// during InitServiceConfig after the built-in validation completes.
//
// Validators receive a read-only copy of the Config and should return an error
// if validation fails. Multiple validators can be registered and they run in order.
//
// Must be called before InitServiceConfig.
//
// Example:
//
//	config.AddValidator(func(cfg config.Config) error {
//	    svc, ok := cfg.Service.(*MyServiceConfig)
//	    if !ok {
//	        return fmt.Errorf("unexpected service config type")
//	    }
//	    if svc.Port < 1024 || svc.Port > 65535 {
//	        return fmt.Errorf("port must be between 1024 and 65535, got %d", svc.Port)
//	    }
//	    return nil
//	})
func AddValidator(fn ValidatorFunc) {
	mu.Lock()
	defer mu.Unlock()

	globalConfig.validators = append(globalConfig.validators, fn)
}

// GetSecureCopy returns a copy of the configuration with sensitive values masked.
//
// It masks the base AppSecret field and any fields tagged with `sensitive:"true"`
// in the service configuration struct.
//
// This is useful for logging or displaying the configuration without exposing
// sensitive information like secrets or passwords.
//
// Example:
//
//	secureCfg := config.GetSecureCopy()
//	fmt.Printf("%+v\n", secureCfg)
func GetSecureCopy() Config {
	mu.RLock()
	defer mu.RUnlock()

	return secureCopyLocked()
}

// LogConfig logs the configuration at debug level, masking sensitive values.
//
// This is a convenience method for safely logging the configuration.
//
// Example:
//
//	config.LogConfig()
func LogConfig() {
	mu.RLock()
	defer mu.RUnlock()

	logConfigLocked()
}

// DefaultConfig generates a base configuration file with random credentials and
// zeroed service configuration for a given type.
//
// It should be used to bootstrap a config.yaml with sensible defaults.
//
// Example:
//
//	err := config.DefaultConfig[*MyServiceConfig]("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
func DefaultConfig[T any](configPath string) error {
	mu.Lock()
	defer mu.Unlock()

	var zero T

	globalConfig.Service = zero

	return defaultConfig(configPath)
}

// WatchConfig starts watching the configuration file for changes.
// When the file is modified, it is automatically re-read and the global
// configuration is updated. The optional onChange callback is invoked
// after each successful reload.
//
// WatchConfig must be called after InitServiceConfig. It launches a
// background goroutine and returns immediately.
//
// Example:
//
//	config.WatchConfig(func() {
//	    log.Println("config reloaded")
//	})
func WatchConfig(onChange ...func()) {
	mu.RLock()
	v := globalConfig.viper
	afs := afero.NewOsFs()
	mu.RUnlock()

	v.OnConfigChange(func(_ fsnotify.Event) {
		mu.Lock()
		defer mu.Unlock()

		if err := v.Unmarshal(globalConfig); err != nil {
			slog.Error("failed to unmarshal config after reload", "error", err)
			return
		}

		if err := globalConfig.loadProfile(afs); err != nil {
			slog.Error("failed to load profile after reload", "error", err)
			return
		}

		if err := globalConfig.runValidators(); err != nil {
			slog.Error("config validation failed after reload", "error", err)
			return
		}

		slog.Info("Configuration reloaded")
		logConfigLocked()

		for _, fn := range onChange {
			fn()
		}
	})

	v.WatchConfig()
}

func secureCopyLocked() Config {
	configClone := *globalConfig

	if configClone.AppSecret != "" {
		configClone.AppSecret = maskedValue
	}

	configClone.Service = maskSensitiveFields(configClone.Service)

	return configClone
}

func logConfigLocked() {
	secureCfg := secureCopyLocked()
	slog.Debug("Current configuration",
		"appID", secureCfg.AppID,
		"appSecret", secureCfg.AppSecret,
		"logLevel", secureCfg.Logger.LogLevel,
	)
}

func (c *Config) defaultValues() error {
	// Validate and set default AppID
	if c.AppID == "" {
		c.AppID = uuid.NewString()
		slog.Debug("Generated new AppID", "appID", c.AppID)
	} else if len(c.AppID) < 8 {
		return fmt.Errorf("invalid AppID: must be at least 8 characters long, got %d characters", len(c.AppID))
	}

	// Validate and set default AppSecret
	if c.AppSecret == "" {
		c.AppSecret = uuid.NewString()
		slog.Debug("Generated new AppSecret")
	} else if len(c.AppSecret) < 12 {
		return fmt.Errorf("invalid AppSecret: must be at least 12 characters long, got %d characters", len(c.AppSecret))
	}

	if c.Environment == "" {
		c.Environment = "dev"
	}

	if c.AppVersion == "" {
		c.AppVersion = "0.0.0-development"
	}

	// Validate log level
	switch strings.ToUpper(c.Logger.LogLevel) {
	case "DEBUG", slog.LevelDebug.String():
	case "INFO", slog.LevelInfo.String():
	case "WARN", "WARNING", slog.LevelWarn.String():
	case "ERROR", slog.LevelError.String():
	default:
		return fmt.Errorf("unknown log level: %q (valid values: DEBUG, INFO, WARN, ERROR)", c.Logger.LogLevel)
	}

	return nil
}

func (c *Config) runValidators() error {
	configCopy := *c
	for _, fn := range c.validators {
		if err := fn(configCopy); err != nil {
			return err
		}
	}
	return nil
}

// loadProfile checks for a profile-specific config file and merges its values
// on top of the base config. For example, if Environment is "prod" and the base
// config file is "config.yaml", it looks for "config.prod.yaml" in the same directory.
func (c *Config) loadProfile(afs afero.Fs) error {
	if c.Environment == "" {
		return nil
	}

	dir := filepath.Dir(c.ConfigFile)
	ext := filepath.Ext(c.ConfigFile)
	base := strings.TrimSuffix(filepath.Base(c.ConfigFile), ext)

	profileFile := filepath.Join(dir, base+"."+c.Environment+ext)

	if !exists(afs, profileFile) {
		return nil
	}

	slog.Info("Loading profile config", "profile", c.Environment, "file", profileFile)

	data, err := afero.ReadFile(afs, profileFile)
	if err != nil {
		return fmt.Errorf("reading profile config %s: %w", profileFile, err)
	}

	profileExt := strings.TrimPrefix(ext, ".")
	c.viper.SetConfigType(profileExt)

	if err = c.viper.MergeConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("merging profile config %s: %w", profileFile, err)
	}

	if err = c.viper.Unmarshal(globalConfig); err != nil {
		return fmt.Errorf("unmarshalling profile config: %w", err)
	}

	return nil
}

func (c *Config) getConfigFile() (string, string, error) {
	ext := strings.TrimPrefix(filepath.Ext(c.ConfigFile), ".")
	if !slices.Contains([]string{"json", "yaml", "yml"}, ext) {
		return "", "", fmt.Errorf("unsupported config file extension: %s", ext)
	}

	return c.ConfigFile, ext, nil
}

func (c *Config) readInConfig(afs afero.Fs) error {
	slog.Info("Reading config file", "file", c.ConfigFile)

	filename, ext, err := c.getConfigFile()
	if err != nil {
		return err
	}

	file, err := afero.ReadFile(afs, filename)
	if err != nil {
		return err
	}

	c.viper.SetConfigType(ext)
	c.viper.SetConfigFile(filename)

	// Configure environment variable binding
	if c.envPrefix != "" {
		slog.Debug("Setting environment variable prefix", "prefix", c.envPrefix)
		c.viper.SetEnvPrefix(c.envPrefix)
		c.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	}
	c.viper.AutomaticEnv()

	if err = c.viper.ReadConfig(bytes.NewReader(file)); err != nil {
		return fmt.Errorf("reading config content: %w", err)
	}

	if err = c.viper.Unmarshal(globalConfig); err != nil {
		return fmt.Errorf("unmarshalling config: %w", err)
	}

	return nil
}

// writeToFile writes the global config to the given file path atomically.
// It writes to a temporary file first, then renames to the target path
// to prevent data loss if encoding fails. The encoding format is determined
// by the file extension (JSON for .json, YAML otherwise).
func writeToFile(cfgFile string) error {
	dir := filepath.Dir(cfgFile)

	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	ext := strings.TrimPrefix(filepath.Ext(cfgFile), ".")
	if ext == "json" {
		encoder := json.NewEncoder(tmp)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(globalConfig)
	} else {
		encoder := yaml.NewEncoder(tmp)
		encoder.SetIndent(2)
		err = encoder.Encode(globalConfig)
	}

	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("encoding config: %w", err)
	}

	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err = os.Rename(tmpName, cfgFile); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

func exists(fs afero.Fs, path string) bool {
	stat, err := fs.Stat(path)
	return err == nil && !stat.IsDir()
}

func defaultConfig(configPath string) error {
	if err := globalConfig.defaultValues(); err != nil {
		return err
	}
	return writeToFile(configPath)
}

// maskSensitiveFields returns a copy of v with all fields tagged
// `sensitive:"true"` replaced with "********". If v is not a struct
// pointer, it is returned unchanged.
func maskSensitiveFields(v any) any {
	if v == nil {
		return v
	}

	rv := reflect.ValueOf(v)

	// Dereference pointer
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return v
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return v
	}

	// Create a new instance to avoid mutating the original
	cp := reflect.New(rv.Type()).Elem()
	cp.Set(rv)

	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		if field.Tag.Get("sensitive") == "true" && field.Type.Kind() == reflect.String {
			cpField := cp.Field(i)
			if cpField.CanSet() && cpField.String() != "" {
				cpField.SetString(maskedValue)
			}
		}
	}

	// Return as pointer if original was pointer
	if reflect.ValueOf(v).Kind() == reflect.Ptr {
		ptr := reflect.New(rv.Type())
		ptr.Elem().Set(cp)
		return ptr.Interface()
	}

	return cp.Interface()
}
