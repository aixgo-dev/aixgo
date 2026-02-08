package session

// Config holds session configuration from YAML.
type Config struct {
	// Enabled determines whether sessions are active.
	// Default: true (sessions are enabled by default).
	Enabled bool `yaml:"enabled"`

	// Store specifies the storage backend type.
	// Options: "file", "firestore", "postgres"
	// Default: "file"
	Store string `yaml:"store"`

	// BaseDir is the base directory for file-based storage.
	// Default: ~/.aixgo/sessions
	BaseDir string `yaml:"base_dir"`

	// Checkpoint contains checkpoint configuration.
	Checkpoint CheckpointConfig `yaml:"checkpoint,omitempty"`
}

// CheckpointConfig holds checkpoint-specific settings.
type CheckpointConfig struct {
	// AutoSave enables automatic checkpoint creation.
	AutoSave bool `yaml:"auto_save"`

	// Interval is the auto-save interval (e.g., "5m").
	Interval string `yaml:"interval"`
}

// DefaultConfig returns the default session configuration.
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Store:   "file",
		BaseDir: "",
		Checkpoint: CheckpointConfig{
			AutoSave: false,
			Interval: "5m",
		},
	}
}
