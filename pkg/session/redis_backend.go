package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBackend implements StorageBackend using Redis.
// It provides distributed session storage suitable for multi-node deployments.
type RedisBackend struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
	mu     sync.RWMutex
	closed bool
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	// Addr is the Redis server address (host:port).
	Addr string
	// Password is the Redis password (optional).
	Password string
	// DB is the Redis database number.
	DB int
	// Prefix is the key prefix for all session keys (default: "aixgo:session:").
	Prefix string
	// SessionTTL is the session expiry duration (0 = never expire).
	SessionTTL time.Duration
	// PoolSize is the connection pool size (default: 10).
	PoolSize int
}

// NewRedisBackend creates a new Redis storage backend.
func NewRedisBackend(cfg RedisConfig) (*RedisBackend, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis address is required")
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "aixgo:session:"
	}

	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: poolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// Close client to release connection pool resources
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisBackend{
		client: client,
		prefix: prefix,
		ttl:    cfg.SessionTTL,
	}, nil
}

// NewRedisBackendFromClient creates a Redis backend from an existing client.
// This is useful for testing with miniredis.
func NewRedisBackendFromClient(client *redis.Client, prefix string, ttl time.Duration) *RedisBackend {
	if prefix == "" {
		prefix = "aixgo:session:"
	}
	return &RedisBackend{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

// Key helpers
func (b *RedisBackend) sessionKey(sessionID string) string {
	return b.prefix + "meta:" + sessionID
}

func (b *RedisBackend) entriesKey(sessionID string) string {
	return b.prefix + "entries:" + sessionID
}

func (b *RedisBackend) agentIndexKey(agentName string) string {
	return b.prefix + "agent:" + agentName
}

func (b *RedisBackend) userIndexKey(userID string) string {
	return b.prefix + "user:" + userID
}

func (b *RedisBackend) checkpointKey(checkpointID string) string {
	return b.prefix + "checkpoint:" + checkpointID
}

func (b *RedisBackend) sessionCheckpointsKey(sessionID string) string {
	return b.prefix + "session-checkpoints:" + sessionID
}

// SaveSession creates or updates session metadata.
func (b *RedisBackend) SaveSession(ctx context.Context, meta *SessionMetadata) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	pipe := b.client.Pipeline()

	// Save metadata
	if b.ttl > 0 {
		pipe.Set(ctx, b.sessionKey(meta.ID), data, b.ttl)
	} else {
		pipe.Set(ctx, b.sessionKey(meta.ID), data, 0)
	}

	// Add to agent index
	pipe.SAdd(ctx, b.agentIndexKey(meta.AgentName), meta.ID)

	// Add to user index if user ID provided
	if meta.UserID != "" {
		pipe.SAdd(ctx, b.userIndexKey(meta.UserID), meta.ID)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

// LoadSession retrieves session metadata by ID.
func (b *RedisBackend) LoadSession(ctx context.Context, sessionID string) (*SessionMetadata, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := b.client.Get(ctx, b.sessionKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	var meta SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return &meta, nil
}

// DeleteSession removes a session and all its data.
func (b *RedisBackend) DeleteSession(ctx context.Context, sessionID string) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrStorageClosed
	}
	b.mu.RUnlock()

	// Load metadata to get agent and user info for index cleanup
	meta, err := b.LoadSession(ctx, sessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}

	pipe := b.client.Pipeline()

	// Delete session data
	pipe.Del(ctx, b.sessionKey(sessionID))
	pipe.Del(ctx, b.entriesKey(sessionID))

	// Clean up indexes if metadata was found
	if meta != nil {
		pipe.SRem(ctx, b.agentIndexKey(meta.AgentName), sessionID)
		if meta.UserID != "" {
			pipe.SRem(ctx, b.userIndexKey(meta.UserID), sessionID)
		}
	}

	// Delete checkpoints (ignore SMembers error - checkpoints are optional)
	checkpointIDs, smErr := b.client.SMembers(ctx, b.sessionCheckpointsKey(sessionID)).Result()
	if smErr == nil {
		for _, cpID := range checkpointIDs {
			pipe.Del(ctx, b.checkpointKey(cpID))
		}
	}
	pipe.Del(ctx, b.sessionCheckpointsKey(sessionID))

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// ListSessions returns sessions for an agent matching filter options.
func (b *RedisBackend) ListSessions(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, ErrStorageClosed
	}
	b.mu.RUnlock()

	// Get session IDs from appropriate index
	var sessionIDs []string
	var err error

	if opts.UserID != "" {
		// Intersect agent and user indexes
		sessionIDs, err = b.client.SInter(ctx,
			b.agentIndexKey(agentName),
			b.userIndexKey(opts.UserID),
		).Result()
	} else {
		sessionIDs, err = b.client.SMembers(ctx, b.agentIndexKey(agentName)).Result()
	}

	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// Sort session IDs for deterministic pagination (Redis sets are unordered)
	sort.Strings(sessionIDs)

	// Apply offset and limit
	start := opts.Offset
	if start >= len(sessionIDs) {
		return []*SessionMetadata{}, nil
	}

	end := len(sessionIDs)
	if opts.Limit > 0 && start+opts.Limit < end {
		end = start + opts.Limit
	}

	sessionIDs = sessionIDs[start:end]

	// Load metadata for each session
	sessions := make([]*SessionMetadata, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		meta, err := b.LoadSession(ctx, id)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				// Session was deleted, clean up index
				b.client.SRem(ctx, b.agentIndexKey(agentName), id)
				continue
			}
			return nil, err
		}
		sessions = append(sessions, meta)
	}

	return sessions, nil
}

// AppendEntry adds an entry to a session.
func (b *RedisBackend) AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	if err := b.client.RPush(ctx, b.entriesKey(sessionID), data).Err(); err != nil {
		return fmt.Errorf("append entry: %w", err)
	}

	// Update TTL if configured
	if b.ttl > 0 {
		if err := b.client.Expire(ctx, b.entriesKey(sessionID), b.ttl).Err(); err != nil {
			// Log warning but don't fail the append - entry was already saved
			// The TTL will be applied on the next successful Expire call
			_ = err // Expire failure is non-fatal
		}
	}

	return nil
}

// LoadEntries retrieves all entries for a session in order.
func (b *RedisBackend) LoadEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := b.client.LRange(ctx, b.entriesKey(sessionID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}

	entries := make([]*SessionEntry, 0, len(data))
	for _, d := range data {
		var entry SessionEntry
		if err := json.Unmarshal([]byte(d), &entry); err != nil {
			return nil, fmt.Errorf("unmarshal entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// SaveCheckpoint stores a checkpoint.
func (b *RedisBackend) SaveCheckpoint(ctx context.Context, checkpoint *Checkpoint) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	pipe := b.client.Pipeline()

	// Save checkpoint
	if b.ttl > 0 {
		pipe.Set(ctx, b.checkpointKey(checkpoint.ID), data, b.ttl)
	} else {
		pipe.Set(ctx, b.checkpointKey(checkpoint.ID), data, 0)
	}

	// Add to session's checkpoint index
	pipe.SAdd(ctx, b.sessionCheckpointsKey(checkpoint.SessionID), checkpoint.ID)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint retrieves a checkpoint by ID.
func (b *RedisBackend) LoadCheckpoint(ctx context.Context, checkpointID string) (*Checkpoint, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, ErrStorageClosed
	}
	b.mu.RUnlock()

	data, err := b.client.Get(ctx, b.checkpointKey(checkpointID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// Close releases resources held by the backend.
func (b *RedisBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	return b.client.Close()
}

// Ping checks if the Redis connection is alive.
func (b *RedisBackend) Ping(ctx context.Context) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrStorageClosed
	}
	b.mu.RUnlock()

	return b.client.Ping(ctx).Err()
}
