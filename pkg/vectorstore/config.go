package vectorstore

import "fmt"

// Config holds configuration for vector store providers.
type Config struct {
	// Provider specifies which vector store to use
	// Supported values: "firestore", "qdrant", "pgvector", "memory"
	Provider string `yaml:"provider" json:"provider"`

	// EmbeddingDimensions is the size of the embedding vectors
	// Common values: 384, 768, 1024, 1536, 2048
	EmbeddingDimensions int `yaml:"embedding_dimensions" json:"embedding_dimensions"`

	// DefaultTopK is the default number of results to return
	DefaultTopK int `yaml:"default_top_k" json:"default_top_k"`

	// DefaultDistanceMetric is the default similarity metric
	// Values: "cosine", "euclidean", "dot_product"
	DefaultDistanceMetric string `yaml:"default_distance_metric" json:"default_distance_metric"`

	// Firestore-specific configuration
	Firestore *FirestoreConfig `yaml:"firestore,omitempty" json:"firestore,omitempty"`

	// Qdrant-specific configuration (future)
	Qdrant *QdrantConfig `yaml:"qdrant,omitempty" json:"qdrant,omitempty"`

	// PgVector-specific configuration (future)
	PgVector *PgVectorConfig `yaml:"pgvector,omitempty" json:"pgvector,omitempty"`

	// Memory-specific configuration (for testing)
	Memory *MemoryConfig `yaml:"memory,omitempty" json:"memory,omitempty"`
}

// FirestoreConfig contains Firestore-specific settings.
type FirestoreConfig struct {
	// ProjectID is the Google Cloud project ID
	ProjectID string `yaml:"project_id" json:"project_id"`

	// Collection is the Firestore collection name for documents
	Collection string `yaml:"collection" json:"collection"`

	// CredentialsFile is the path to the service account key JSON file
	// Optional: uses Application Default Credentials if not specified
	CredentialsFile string `yaml:"credentials_file,omitempty" json:"credentials_file,omitempty"`

	// DatabaseID is the Firestore database ID (default: "(default)")
	DatabaseID string `yaml:"database_id,omitempty" json:"database_id,omitempty"`
}

// QdrantConfig contains Qdrant-specific settings (future implementation).
type QdrantConfig struct {
	// Host is the Qdrant server host
	Host string `yaml:"host" json:"host"`

	// Port is the Qdrant server port
	Port int `yaml:"port" json:"port"`

	// APIKey for authentication (optional)
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`

	// Collection is the Qdrant collection name
	Collection string `yaml:"collection" json:"collection"`

	// UseTLS enables TLS connection
	UseTLS bool `yaml:"use_tls" json:"use_tls"`
}

// PgVectorConfig contains pgvector-specific settings (future implementation).
type PgVectorConfig struct {
	// ConnectionString is the PostgreSQL connection string
	// Example: "postgresql://user:password@localhost:5432/dbname"
	ConnectionString string `yaml:"connection_string" json:"connection_string"`

	// Table is the table name for storing vectors
	Table string `yaml:"table" json:"table"`

	// IndexType specifies the vector index type
	// Values: "ivfflat", "hnsw"
	IndexType string `yaml:"index_type" json:"index_type"`

	// MaxConnections is the maximum number of database connections
	MaxConnections int `yaml:"max_connections" json:"max_connections"`
}

// MemoryConfig contains in-memory store settings (for testing).
type MemoryConfig struct {
	// MaxDocuments is the maximum number of documents to store (default: 10000)
	MaxDocuments int `yaml:"max_documents" json:"max_documents"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must be specified")
	}

	// Validate embedding dimensions
	if c.EmbeddingDimensions < 1 || c.EmbeddingDimensions > 4096 {
		return fmt.Errorf("embedding_dimensions must be between 1 and 4096, got %d", c.EmbeddingDimensions)
	}

	// Set defaults
	if c.DefaultTopK == 0 {
		c.DefaultTopK = 10
	}
	if c.DefaultTopK < 1 || c.DefaultTopK > 1000 {
		return fmt.Errorf("default_top_k must be between 1 and 1000, got %d", c.DefaultTopK)
	}

	if c.DefaultDistanceMetric == "" {
		c.DefaultDistanceMetric = string(DistanceMetricCosine)
	}

	// Validate provider-specific configuration
	switch c.Provider {
	case "firestore":
		if c.Firestore == nil {
			return fmt.Errorf("firestore configuration is required when provider is 'firestore'")
		}
		return c.Firestore.Validate()
	case "qdrant":
		if c.Qdrant == nil {
			return fmt.Errorf("qdrant configuration is required when provider is 'qdrant'")
		}
		return c.Qdrant.Validate()
	case "pgvector":
		if c.PgVector == nil {
			return fmt.Errorf("pgvector configuration is required when provider is 'pgvector'")
		}
		return c.PgVector.Validate()
	case "memory":
		if c.Memory == nil {
			c.Memory = &MemoryConfig{MaxDocuments: 10000}
		}
		return c.Memory.Validate()
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

// Validate checks if Firestore configuration is valid.
func (fc *FirestoreConfig) Validate() error {
	if fc.ProjectID == "" {
		return fmt.Errorf("firestore project_id is required")
	}
	if fc.Collection == "" {
		return fmt.Errorf("firestore collection is required")
	}
	return nil
}

// Validate checks if Qdrant configuration is valid.
func (qc *QdrantConfig) Validate() error {
	if qc.Host == "" {
		return fmt.Errorf("qdrant host is required")
	}
	if qc.Port < 1 || qc.Port > 65535 {
		return fmt.Errorf("qdrant port must be between 1 and 65535, got %d", qc.Port)
	}
	if qc.Collection == "" {
		return fmt.Errorf("qdrant collection is required")
	}
	return nil
}

// Validate checks if PgVector configuration is valid.
func (pc *PgVectorConfig) Validate() error {
	if pc.ConnectionString == "" {
		return fmt.Errorf("pgvector connection_string is required")
	}
	if pc.Table == "" {
		return fmt.Errorf("pgvector table is required")
	}
	if pc.MaxConnections < 1 {
		pc.MaxConnections = 10 // Default
	}
	return nil
}

// Validate checks if Memory configuration is valid.
func (mc *MemoryConfig) Validate() error {
	if mc.MaxDocuments < 1 {
		mc.MaxDocuments = 10000
	}
	return nil
}
