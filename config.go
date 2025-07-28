package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var globalConfig *Config

func init() {
	// viper.SetOptions()

	globalConfig = &Config{
		viper: viper.NewWithOptions(),
		Logger: Logger{
			LogLevel: slog.LevelDebug.String(),
		},
	}
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
	AppSecret  string `yaml:"appSecret" mapstructure:"appSecret"`
	Logger     Logger `yaml:"logger" mapstructure:"logger"`
	Service    any    `yaml:"service" mapstructure:"service"`
}

// InitServiceConfig loads a configuration file and binds a service-specific
// struct to the `Service` field in the global config.
//
// It must be called before accessing the service configuration via GetServiceConfig.
//
// Example:
//
//	type MyServiceConfig struct {
//	    Port int
//	    Mode string
//	}
//
//	err := config.InitServiceConfig(&MyServiceConfig{}, "config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
func InitServiceConfig(v any, configPath string) error {
	afs := afero.NewOsFs()

	configFile, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	if !exists(afs, configFile) {
		globalConfig.Init = true
		if err := defaultConfig(configPath); err != nil {
			return fmt.Errorf("writing default config: %w", err)
		}
		globalConfig.Init = false
	}

	globalConfig.ConfigFile = configFile
	globalConfig.Service = v

	if err = globalConfig.readInConfig(afs); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	if err = globalConfig.defaultValues(); err != nil {
		return fmt.Errorf("setting default values: %w", err)
	}

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
	if c.AppID == "" {
		c.AppID = uuid.NewString()
	}

	if c.AppSecret == "" {
		c.AppSecret = uuid.NewString()
	}

	opts := &slog.HandlerOptions{}

	switch c.Logger.LogLevel {
	case slog.LevelDebug.String():
		opts.Level = slog.LevelDebug
	case slog.LevelInfo.String():
		opts.Level = slog.LevelInfo
	case slog.LevelWarn.String():
		opts.Level = slog.LevelWarn
	case slog.LevelError.String():
		opts.Level = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %s", c.Logger.LogLevel)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
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
