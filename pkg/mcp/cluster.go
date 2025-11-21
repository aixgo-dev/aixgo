package mcp

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancerStrategy defines the load balancing algorithm
type LoadBalancerStrategy string

const (
	RoundRobin       LoadBalancerStrategy = "round-robin"
	LeastConnections LoadBalancerStrategy = "least-connections"
	Random           LoadBalancerStrategy = "random"
	WeightedRandom   LoadBalancerStrategy = "weighted-random"
)

// NodeState represents the state of a cluster node
type NodeState string

const (
	NodeStateHealthy   NodeState = "healthy"
	NodeStateUnhealthy NodeState = "unhealthy"
	NodeStateDraining  NodeState = "draining"
	NodeStateRemoved   NodeState = "removed"
)

// ClusterNode represents a node in the cluster
type ClusterNode struct {
	mu              sync.RWMutex
	Instance        *ServiceInstance
	State           NodeState
	LastHealthCheck time.Time
	FailureCount    int
	Connections     int64
	Transport       *GRPCTransport
}

// IncrementConnections atomically increments the connection count
func (n *ClusterNode) IncrementConnections() {
	atomic.AddInt64(&n.Connections, 1)
}

// DecrementConnections atomically decrements the connection count
func (n *ClusterNode) DecrementConnections() {
	atomic.AddInt64(&n.Connections, -1)
}

// GetConnections atomically returns the connection count
func (n *ClusterNode) GetConnections() int64 {
	return atomic.LoadInt64(&n.Connections)
}

// ClusterConfig configures the cluster coordinator
type ClusterConfig struct {
	ServiceName         string
	Discovery           ServiceDiscovery
	Strategy            LoadBalancerStrategy
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	MaxFailures         int
	RetryAttempts       int
	RetryDelay          time.Duration
	TLS                 *TLSConfig
}

// Cluster coordinates multiple service nodes with load balancing and failover
type Cluster struct {
	mu       sync.RWMutex
	config   ClusterConfig
	nodes    map[string]*ClusterNode
	nodeList []*ClusterNode // For round-robin iteration
	rrIndex  uint64
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
}

// NewCluster creates a new cluster coordinator
func NewCluster(config ClusterConfig) (*Cluster, error) {
	if config.Discovery == nil {
		return nil, errors.New("discovery is required")
	}
	if config.ServiceName == "" {
		return nil, errors.New("service name is required")
	}

	if config.Strategy == "" {
		config.Strategy = RoundRobin
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 5 * time.Second
	}
	if config.MaxFailures == 0 {
		config.MaxFailures = 3
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 100 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Cluster{
		config:   config,
		nodes:    make(map[string]*ClusterNode),
		nodeList: make([]*ClusterNode, 0),
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start initializes the cluster and begins health checking
func (c *Cluster) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.mu.Unlock()

	// Initial discovery
	if err := c.refreshNodes(ctx); err != nil {
		return fmt.Errorf("initial discovery failed: %w", err)
	}

	// Start background health checking
	go c.healthCheckLoop()

	// Start watching for service changes
	go c.watchServices()

	return nil
}

// Stop gracefully shuts down the cluster
func (c *Cluster) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}

	c.cancel()
	c.started = false

	// Close all node transports
	for _, node := range c.nodes {
		if node.Transport != nil {
			_ = node.Transport.Close()
		}
	}

	return nil
}

// GetNode returns a node using the configured load balancing strategy
func (c *Cluster) GetNode() (*ClusterNode, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	healthyNodes := c.getHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil, errors.New("no healthy nodes available")
	}

	switch c.config.Strategy {
	case RoundRobin:
		return c.roundRobinSelect(healthyNodes), nil
	case LeastConnections:
		return c.leastConnectionsSelect(healthyNodes), nil
	case Random:
		return c.randomSelect(healthyNodes), nil
	case WeightedRandom:
		return c.weightedRandomSelect(healthyNodes), nil
	default:
		return c.roundRobinSelect(healthyNodes), nil
	}
}

// Send sends a request with automatic failover
func (c *Cluster) Send(ctx context.Context, method string, params any) (any, error) {
	var lastErr error

	for attempt := 0; attempt < c.config.RetryAttempts; attempt++ {
		node, err := c.GetNode()
		if err != nil {
			return nil, err
		}

		node.IncrementConnections()
		result, err := c.sendToNode(ctx, node, method, params)
		node.DecrementConnections()

		if err == nil {
			return result, nil
		}

		lastErr = err
		c.handleNodeFailure(node)

		if attempt < c.config.RetryAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}
	}

	return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// RegisterNode manually registers a node
func (c *Cluster) RegisterNode(instance *ServiceInstance) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.nodes[instance.ID]; exists {
		return nil
	}

	node := &ClusterNode{
		Instance:        instance,
		State:           NodeStateHealthy,
		LastHealthCheck: time.Now(),
	}

	c.nodes[instance.ID] = node
	c.nodeList = append(c.nodeList, node)

	return nil
}

// DeregisterNode removes a node from the cluster
func (c *Cluster) DeregisterNode(instanceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.nodes[instanceID]
	if !exists {
		return fmt.Errorf("node %s not found", instanceID)
	}

	node.State = NodeStateRemoved
	delete(c.nodes, instanceID)

	// Remove from nodeList
	for i, n := range c.nodeList {
		if n.Instance.ID == instanceID {
			c.nodeList = append(c.nodeList[:i], c.nodeList[i+1:]...)
			break
		}
	}

	if node.Transport != nil {
		_ = node.Transport.Close()
	}

	return nil
}

// Nodes returns all registered nodes
func (c *Cluster) Nodes() []*ClusterNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*ClusterNode, len(c.nodeList))
	copy(result, c.nodeList)
	return result
}

// HealthyNodes returns only healthy nodes
func (c *Cluster) HealthyNodes() []*ClusterNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getHealthyNodes()
}

// internal methods

func (c *Cluster) getHealthyNodes() []*ClusterNode {
	healthy := make([]*ClusterNode, 0, len(c.nodeList))
	for _, node := range c.nodeList {
		if node.State == NodeStateHealthy {
			healthy = append(healthy, node)
		}
	}
	return healthy
}

func (c *Cluster) roundRobinSelect(nodes []*ClusterNode) *ClusterNode {
	idx := atomic.AddUint64(&c.rrIndex, 1)
	return nodes[idx%uint64(len(nodes))]
}

func (c *Cluster) leastConnectionsSelect(nodes []*ClusterNode) *ClusterNode {
	var selected *ClusterNode
	minConns := int64(^uint64(0) >> 1) // Max int64

	for _, node := range nodes {
		conns := node.GetConnections()
		if conns < minConns {
			minConns = conns
			selected = node
		}
	}

	return selected
}

func (c *Cluster) randomSelect(nodes []*ClusterNode) *ClusterNode {
	return nodes[rand.Intn(len(nodes))]
}

func (c *Cluster) weightedRandomSelect(nodes []*ClusterNode) *ClusterNode {
	totalWeight := 0
	for _, node := range nodes {
		weight := node.Instance.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	r := rand.Intn(totalWeight)
	for _, node := range nodes {
		weight := node.Instance.Weight
		if weight <= 0 {
			weight = 1
		}
		r -= weight
		if r < 0 {
			return node
		}
	}

	return nodes[0]
}

func (c *Cluster) sendToNode(ctx context.Context, node *ClusterNode, method string, params any) (any, error) {
	node.mu.Lock()
	if node.Transport == nil {
		grpcConfig := GRPCTransportConfig{
			Address: node.Instance.Endpoint(),
			TLS:     c.config.TLS,
		}
		transport, err := NewGRPCTransportWithConfig(grpcConfig)
		if err != nil {
			node.mu.Unlock()
			return nil, err
		}
		node.Transport = transport
	}
	transport := node.Transport
	node.mu.Unlock()

	if err := transport.Connect(ctx); err != nil {
		return nil, err
	}

	return transport.Send(ctx, method, params)
}

func (c *Cluster) handleNodeFailure(node *ClusterNode) {
	node.mu.Lock()
	defer node.mu.Unlock()

	node.FailureCount++
	if node.FailureCount >= c.config.MaxFailures {
		node.State = NodeStateUnhealthy
		node.Instance.Healthy = false
	}
}

func (c *Cluster) refreshNodes(ctx context.Context) error {
	instances, err := c.config.Discovery.Discover(ctx, c.config.ServiceName)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Track existing nodes
	existing := make(map[string]bool)
	for id := range c.nodes {
		existing[id] = true
	}

	// Add or update nodes
	for _, inst := range instances {
		if node, exists := c.nodes[inst.ID]; exists {
			node.Instance = inst
			if inst.Healthy {
				node.State = NodeStateHealthy
			}
			delete(existing, inst.ID)
		} else {
			node := &ClusterNode{
				Instance:        inst,
				State:           NodeStateHealthy,
				LastHealthCheck: time.Now(),
			}
			c.nodes[inst.ID] = node
			c.nodeList = append(c.nodeList, node)
		}
	}

	// Mark removed nodes
	for id := range existing {
		if node, ok := c.nodes[id]; ok {
			node.State = NodeStateRemoved
		}
	}

	return nil
}

func (c *Cluster) healthCheckLoop() {
	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.performHealthChecks()
		}
	}
}

func (c *Cluster) performHealthChecks() {
	c.mu.RLock()
	nodes := make([]*ClusterNode, len(c.nodeList))
	copy(nodes, c.nodeList)
	c.mu.RUnlock()

	for _, node := range nodes {
		if node.State == NodeStateRemoved {
			continue
		}

		ctx, cancel := context.WithTimeout(c.ctx, c.config.HealthCheckTimeout)
		healthy := c.checkNodeHealth(ctx, node)
		cancel()

		node.mu.Lock()
		node.LastHealthCheck = time.Now()
		if healthy {
			node.FailureCount = 0
			node.State = NodeStateHealthy
			node.Instance.Healthy = true
		} else {
			node.FailureCount++
			if node.FailureCount >= c.config.MaxFailures {
				node.State = NodeStateUnhealthy
				node.Instance.Healthy = false
			}
		}
		node.mu.Unlock()
	}
}

func (c *Cluster) checkNodeHealth(ctx context.Context, node *ClusterNode) bool {
	node.mu.Lock()
	if node.Transport == nil {
		grpcConfig := GRPCTransportConfig{
			Address: node.Instance.Endpoint(),
			TLS:     c.config.TLS,
		}
		transport, err := NewGRPCTransportWithConfig(grpcConfig)
		if err != nil {
			node.mu.Unlock()
			return false
		}
		node.Transport = transport
	}
	transport := node.Transport
	node.mu.Unlock()

	err := transport.Connect(ctx)
	return err == nil
}

func (c *Cluster) watchServices() {
	watchCh, err := c.config.Discovery.Watch(c.ctx, c.config.ServiceName)
	if err != nil {
		return
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case instances, ok := <-watchCh:
			if !ok {
				return
			}
			c.updateFromDiscovery(instances)
		}
	}
}

func (c *Cluster) updateFromDiscovery(instances []*ServiceInstance) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing := make(map[string]bool)
	for id := range c.nodes {
		existing[id] = true
	}

	for _, inst := range instances {
		if node, exists := c.nodes[inst.ID]; exists {
			node.Instance = inst
			delete(existing, inst.ID)
		} else {
			node := &ClusterNode{
				Instance:        inst,
				State:           NodeStateHealthy,
				LastHealthCheck: time.Now(),
			}
			c.nodes[inst.ID] = node
			c.nodeList = append(c.nodeList, node)
		}
	}

	for id := range existing {
		if node, ok := c.nodes[id]; ok {
			node.State = NodeStateRemoved
		}
	}
}

// ClusterTransport wraps a Cluster to implement the Transport interface
type ClusterTransport struct {
	cluster *Cluster
}

// NewClusterTransport creates a Transport backed by cluster coordination
func NewClusterTransport(cluster *Cluster) *ClusterTransport {
	return &ClusterTransport{cluster: cluster}
}

// Send sends a request through the cluster with load balancing and failover
func (t *ClusterTransport) Send(ctx context.Context, method string, params any) (any, error) {
	return t.cluster.Send(ctx, method, params)
}

// Close stops the cluster
func (t *ClusterTransport) Close() error {
	return t.cluster.Stop()
}

// Interface compliance
var _ Transport = (*ClusterTransport)(nil)
