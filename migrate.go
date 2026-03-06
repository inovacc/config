package config

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// MigrationFunc transforms the configuration from one version to the next.
// It receives a mutable map of the raw config data and should modify it
// in place (e.g., rename keys, change structure).
type MigrationFunc func(data map[string]any) error

type migration struct {
	from int
	to   int
	fn   MigrationFunc
}

// AddMigration registers a migration function that transforms config data
// from version `from` to version `to`. Migrations are run in order during
// InitServiceConfig when the config file's version is older than the
// target version.
//
// Example:
//
//	config.AddMigration(1, 2, func(data map[string]any) error {
//	    // Rename "logLevel" to "log_level" in logger section
//	    if logger, ok := data["logger"].(map[string]any); ok {
//	        if level, exists := logger["logLevel"]; exists {
//	            logger["log_level"] = level
//	            delete(logger, "logLevel")
//	        }
//	    }
//	    data["version"] = 2
//	    return nil
//	})
func AddMigration(from, to int, fn MigrationFunc) {
	mu.Lock()
	defer mu.Unlock()

	globalConfig.migrations = append(globalConfig.migrations, migration{
		from: from,
		to:   to,
		fn:   fn,
	})
}

// SetTargetVersion sets the expected config version. During InitServiceConfig,
// if the config file's version is lower than the target, registered migrations
// are applied in order. If no target is set, migrations are skipped.
//
// Example:
//
//	config.SetTargetVersion(3)
func SetTargetVersion(version int) {
	mu.Lock()
	defer mu.Unlock()

	globalConfig.targetVersion = version
}

// GetConfigVersion returns the current version from the loaded configuration.
func GetConfigVersion() int {
	mu.RLock()
	defer mu.RUnlock()

	return globalConfig.Version
}

// runMigrations applies registered migrations to bring the config from its
// current version up to the target version. Returns true if any migrations
// were applied (meaning the Viper instance needs to be re-read).
func (c *Config) runMigrations() (bool, error) {
	if c.targetVersion == 0 || c.Version >= c.targetVersion {
		return false, nil
	}

	if len(c.migrations) == 0 {
		return false, nil
	}

	// Sort migrations by from-version
	sorted := make([]migration, len(c.migrations))
	copy(sorted, c.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].from < sorted[j].from
	})

	// Get raw config data from Viper
	data := c.viper.AllSettings()

	currentVersion := c.Version
	applied := false

	for _, m := range sorted {
		if m.from != currentVersion {
			continue
		}
		if m.to > c.targetVersion {
			break
		}

		if err := m.fn(data); err != nil {
			return false, fmt.Errorf("migration v%d→v%d: %w", m.from, m.to, err)
		}

		currentVersion = m.to
		applied = true

		if currentVersion >= c.targetVersion {
			break
		}
	}

	if !applied {
		return false, nil
	}

	// Re-read migrated data into Viper at the config level (not override)
	// so that profile merges can still take precedence.
	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(data); err != nil {
		return false, fmt.Errorf("encoding migrated data: %w", err)
	}

	c.viper.SetConfigType("yaml")
	if err := c.viper.ReadConfig(&buf); err != nil {
		return false, fmt.Errorf("re-reading migrated config: %w", err)
	}

	if err := c.viper.Unmarshal(c); err != nil {
		return false, fmt.Errorf("unmarshalling after migration: %w", err)
	}

	return true, nil
}
