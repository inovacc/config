package config

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	appName  = "app"
	CfgFile  string
	instance *Config
	once     sync.Once
)

func init() {
	once.Do(func() {
		instance = &Config{
			fs:            afero.NewOsFs(),
			configPaths:   make([]string, 0),
			supportedExts: []string{"json", "yaml", "yml"},
			Logger: LoggerConfig{
				LogLevel:   slog.LevelDebug.String(),
				LogFormat:  "json",
				MaxSize:    100,
				MaxAge:     7,
				MaxBackups: 10,
				LocalTime:  true,
				Compress:   true,
			},
		}
	})

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

type LoggerConfig struct {
	LogLevel   string `yaml:"logLevel" mapstructure:"logLevel" json:"logLevel"`
	LogFormat  string `yaml:"logFormat" mapstructure:"logFormat" json:"logFormat"`
	FileName   string `yaml:"fileName" mapstructure:"fileName" json:"fileName"`
	MaxSize    int    `yaml:"maxSize" mapstructure:"maxSize" json:"maxSize"`
	MaxAge     int    `yaml:"maxAge" mapstructure:"maxAge" json:"maxAge"`
	MaxBackups int    `yaml:"maxBackups" mapstructure:"maxBackups" json:"maxBackups"`
	LocalTime  bool   `yaml:"localTime" mapstructure:"localTime" json:"localTime"`
	Compress   bool   `yaml:"compress" mapstructure:"compress" json:"compress"`
}

type Config struct {
	ctx            context.Context
	fs             afero.Fs
	configName     string
	configFile     string
	supportedExts  []string
	configPaths    []string
	onConfigChange func(fsnotify.Event)
	watcher        *fsnotify.Watcher
	AppID          string       `yaml:"appID" mapstructure:"appID" json:"appID"`
	AppName        string       `yaml:"appName" mapstructure:"appName" json:"appName"`
	Logger         LoggerConfig `yaml:"logger" mapstructure:"logger" json:"logger"`
	Service        any          `yaml:"service" mapstructure:"service" json:"service"`
}

func (c *Config) defaultValues() error {
	if c.AppName == "" {
		c.AppName = appName
	}

	if c.Logger.FileName == "" {
		c.Logger.FileName = c.AppName
	}

	if c.AppID == "" {
		c.AppID = uuid.NewString()
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
	if c.configFile == "" {
		cf, err := c.findConfigFile()
		if err != nil {
			return "", "", err
		}
		c.configFile = filepath.Clean(cf)
	}

	ext := strings.TrimPrefix(filepath.Ext(c.configFile), ".")
	if !contains(c.supportedExts, ext) {
		return "", "", fmt.Errorf("unsupported config file extension: %s", ext)
	}

	return c.configFile, ext, nil
}

// Find and return a valid configuration file.
func (c *Config) findConfigFile() (string, error) {
	slog.Info("Searching for configuration file", "paths", c.configPaths)
	for _, path := range c.configPaths {
		if file := c.searchInPath(path); file != "" {
			return file, nil
		}
	}
	return "", fmt.Errorf("no config file found in paths: %v", c.configPaths)
}

// Search for a config file in a specified path.
func (c *Config) searchInPath(path string) string {
	for _, ext := range c.supportedExts {
		filePath := filepath.Join(path, fmt.Sprintf("%s.%s", appName, ext))
		if exists(c.fs, filePath) {
			return filePath
		}
	}
	return ""
}

func (c *Config) readInConfig() error {
	slog.Info("attempting to read in config file")
	filename, ext, err := c.getConfigFile()
	if err != nil {
		return err
	}

	slog.Debug("reading file", "file", filename)
	file, err := afero.ReadFile(c.fs, filename)
	if err != nil {
		return err
	}

	viper.SetConfigType(ext)
	viper.SetConfigFile(filename)
	viper.AutomaticEnv()

	if err = viper.ReadConfig(bytes.NewReader(file)); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	if err = viper.Unmarshal(instance); err != nil {
		return fmt.Errorf("fatal error config file: %s", err)
	}

	return nil
}

func DefaultConfig(object any, path string) error {
	if object == nil {
		return fmt.Errorf("need configuration object")
	}

	instance.Service = object

	if err := instance.defaultValues(); err != nil {
		return err
	}
	return writeToFile(filepath.Join(path, "config.yaml"))
}

func GetConfig() *Config {
	return instance
}

func GetConfigContext() context.Context {
	return context.WithValue(instance.ctx, "config", instance)
}

func InitConfigContext(object any) error {
	return InitConfig(context.Background(), object)
}

func InitConfig(ctx context.Context, object any) error {
	if object == nil {
		return fmt.Errorf("need configuration object")
	}

	if CfgFile == "" {
		CfgFile = os.Getenv("CONFIG_FILE")
	}

	configFile, err := filepath.Abs(CfgFile)
	if err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	instance.configFile = configFile

	if err = instance.readInConfig(); err != nil {
		return fmt.Errorf("read in config: %s", err)
	}

	if err = instance.defaultValues(); err != nil {
		return fmt.Errorf("default values: %s", err)
	}

	instance.Service = object
	instance.ctx = ctx

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
	return encoder.Encode(instance)
}

// Check if a file exists.
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
