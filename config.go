package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dyammarcano/config/internal/viper"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

var globalConfig *Config

func init() {
	level := slog.LevelDebug

	globalConfig = &Config{
		Logger: Logger{
			LogLevel: level.String(),
		},
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))

	globalConfig.viper = viper.New()
}

// Logger defines the configuration for structured logging.
type Logger struct {
	LogLevel string `yaml:"logLevel" mapstructure:"logLevel"`
}

// Config represents the global application configuration, including base
// metadata and a generic field for service-specific configuration.
type Config struct {
	viper      *viper.Viper
	ConfigFile string `yaml:"-" mapstructure:"-"`
	Init       bool   `yaml:"-" mapstructure:"-"`
	AppID      string `yaml:"appID" mapstructure:"appID"`
	AppSecret  string `yaml:"appSecret" mapstructure:"appSecret" sensitive:"true"`
	Logger     Logger `yaml:"logger" mapstructure:"logger"`
	Service    any    `yaml:"service" mapstructure:"service"`
	envPrefix  string
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
	afs := afero.NewOsFs()

	configFile, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	globalConfig.ConfigFile = configFile
	globalConfig.Service = v

	// Check if a config file exists, create default if not
	if !exists(afs, configFile) {
		slog.Info("Configuration file not found, creating default", "path", configFile)
		globalConfig.Init = true
		if err := defaultConfig(configPath); err != nil {
			return fmt.Errorf("writing default config: %w", err)
		}
		globalConfig.Init = false
	}

	// Read configuration from a file
	if err = globalConfig.readInConfig(afs); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	// Set default values and configure logging
	if err = globalConfig.defaultValues(); err != nil {
		return fmt.Errorf("setting default values: %w", err)
	}

	// Log the configuration (safely masking sensitive values)
	LogConfig()

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
	var zero T
	val, ok := globalConfig.Service.(T)
	if !ok {
		return zero, fmt.Errorf("invalid service config type: expected %T, got %T", zero, globalConfig.Service)
	}
	return val, nil
}

// GetBaseConfig returns a pointer to the global configuration base object.
//
// This allows access to common fields like AppID, Logger, and AppSecret.
//
// Example:
//
//	cfg := config.GetBaseConfig()
//	fmt.Println("AppID:", cfg.AppID)
func GetBaseConfig() *Config {
	return globalConfig
}

// SetEnvPrefix sets a prefix for environment variables.
//
// Environment variables that match the pattern {prefix}_* will override
// the corresponding configuration values. The matching is case-insensitive.
//
// For example, if the prefix is "APP", then the environment variable "APP_LOGGER_LOGLEVEL"
// will override the value of "logger.logLevel" in the configuration file.
//
// Example:
//
//	config.SetEnvPrefix("APP")
func SetEnvPrefix(prefix string) {
	globalConfig.envPrefix = prefix
}

// GetSecureCopy returns a copy of the configuration with sensitive values masked.
//
// This is useful for logging or displaying the configuration without exposing
// sensitive information like secrets or passwords.
//
// Example:
//
//	secureCfg := config.GetSecureCopy()
//	fmt.Printf("%+v\n", secureCfg)
func GetSecureCopy() Config {
	// Create a configClone of the global config
	configClone := *globalConfig

	// Mask sensitive fields
	if configClone.AppSecret != "" {
		configClone.AppSecret = "********"
	}

	// If the service config has sensitive fields, we should handle them to
	// This requires reflection to find fields with the sensitive tag
	return configClone
}

// LogConfig logs the configuration at debug level, masking sensitive values.
//
// This is a convenience method for safely logging the configuration.
//
// Example:
//
//	config.LogConfig()
func LogConfig() {
	secureCfg := GetSecureCopy()
	slog.Debug("Current configuration",
		"appID", secureCfg.AppID,
		"appSecret", secureCfg.AppSecret,
		"logLevel", secureCfg.Logger.LogLevel,
	)
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
	var zero T

	globalConfig.Init = true
	globalConfig.Service = zero

	return defaultConfig(configPath)
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

	// Configure logging
	opts := &slog.HandlerOptions{}

	switch strings.ToUpper(c.Logger.LogLevel) {
	case "DEBUG", slog.LevelDebug.String():
		opts.Level = slog.LevelDebug
	case "INFO", slog.LevelInfo.String():
		opts.Level = slog.LevelInfo
	case "WARN", "WARNING", slog.LevelWarn.String():
		opts.Level = slog.LevelWarn
	case "ERROR", slog.LevelError.String():
		opts.Level = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %q (valid values: DEBUG, INFO, WARN, ERROR)", c.Logger.LogLevel)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
	slog.Debug("logger configured", "level", c.Logger.LogLevel)
	return nil
}

func (c *Config) getConfigFile() (string, string, error) {
	ext := strings.TrimPrefix(filepath.Ext(c.ConfigFile), ".")
	if !contains([]string{"json", "yaml", "yml"}, ext) {
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

func writeToFile(cfgFile string) error {
	file, err := os.Create(cfgFile)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			slog.Error("error closing config file", slog.String("error", err.Error()))
		}
	}(file)

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	return encoder.Encode(globalConfig)
}

func exists(fs afero.Fs, path string) bool {
	stat, err := fs.Stat(path)
	return err == nil && !stat.IsDir()
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func defaultConfig(configPath string) error {
	if err := globalConfig.defaultValues(); err != nil {
		return err
	}
	return writeToFile(configPath)
}
