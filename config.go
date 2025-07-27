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

var globalConfig = &Config{
	Logger: Logger{
		LogLevel: slog.LevelDebug.String(),
	},
}

type Logger struct {
	LogLevel string `yaml:"logLevel" mapstructure:"logLevel"`
}

type Config struct {
	ConfigFile string `yaml:"-" mapstructure:"-"`
	Init       bool   `yaml:"-" mapstructure:"-"`
	AppID      string `yaml:"appID" mapstructure:"appID"`
	AppSecret  string `yaml:"appSecret" mapstructure:"appSecret"`
	Logger     Logger `yaml:"logger" mapstructure:"logger"`
	Service    any    `yaml:"service" mapstructure:"service"`
}

// SetServiceConfig sets the service-specific configuration struct into the global config.
//
// This function is intended to allow services to register their own configuration type,
// which is stored in the generic `Service` field of the global configuration.
//
// The type of the configuration struct can be anything, typically a pointer to a
// custom struct defined by the consuming service.
//
// Example:
//
//	type MyServiceConfig struct {
//	    Port int
//		Mode string
//	}
//
//	core.SetServiceConfig(&MyServiceConfig{
//		Port: 8080,
//		Mode: "debug",
//	}, "config.yaml")
func SetServiceConfig(v any, configPath string) error {
	afs := afero.NewOsFs()

	configFile, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	if !exists(afs, configFile) {
		return fmt.Errorf("config file does not exist: %s", configFile)
	}

	globalConfig.ConfigFile = configFile
	globalConfig.Service = v

	if err = globalConfig.readInConfig(afs); err != nil {
		return fmt.Errorf("read in config: %s", err)
	}

	if err = globalConfig.defaultValues(); err != nil {
		return fmt.Errorf("default values: %s", err)
	}

	return nil
}

// GetServiceConfig retrieves the service-specific configuration previously registered
// using SetServiceConfig. It uses generics to ensure type safety.
//
// It returns the configuration as the expected type `T`, or an error if the stored type
// does not match the expected type.
//
// Example:
//
//	cfg, err := core.GetServiceConfig[*MyServiceConfig]()
//	if err != nil {
//	    log.Fatalf("config error: %v", err)
//	}
//	fmt.Println("Port:", cfg.Port)
//
// Note: It is the caller’s responsibility to ensure the correct type is requested.
func GetServiceConfig[T any]() (T, error) {
	var zero T
	val, ok := globalConfig.Service.(T)
	if !ok {
		return zero, fmt.Errorf("invalid service config type, expected: %T got: %T", zero, globalConfig.Service)
	}
	return val, nil
}

func DefaultConfig[T any](configPath string) error {
	var zero T

	globalConfig.Init = true
	globalConfig.Service = zero

	if err := globalConfig.defaultValues(); err != nil {
		return err
	}
	return writeToFile(configPath)
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
		c.Logger.LogLevel = slog.LevelDebug.String()
		opts.Level = slog.LevelDebug
	case slog.LevelInfo.String():
		c.Logger.LogLevel = slog.LevelInfo.String()
		opts.Level = slog.LevelInfo
	case slog.LevelWarn.String():
		c.Logger.LogLevel = slog.LevelWarn.String()
		opts.Level = slog.LevelWarn
	case slog.LevelError.String():
		c.Logger.LogLevel = slog.LevelError.String()
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
	slog.Info("attempting to read in config file")
	filename, ext, err := c.getConfigFile()
	if err != nil {
		return err
	}

	slog.Debug("reading file", "file", filename)
	file, err := afero.ReadFile(afs, filename)
	if err != nil {
		return err
	}

	viper.SetConfigType(ext)
	viper.SetConfigFile(filename)
	viper.AutomaticEnv()

	if err = viper.ReadConfig(bytes.NewReader(file)); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	if err = viper.Unmarshal(globalConfig); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	return nil
}

func writeToFile(cfgFile string) error {
	file, err := os.Create(cfgFile)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
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
