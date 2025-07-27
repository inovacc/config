package config

import (
	"os"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

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
