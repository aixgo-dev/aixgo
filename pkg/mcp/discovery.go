package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ServiceInstance represents a discovered service instance
type ServiceInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Healthy  bool              `json:"healthy"`
	Weight   int               `json:"weight,omitempty"`
}

// Endpoint returns the full address:port string
func (s *ServiceInstance) Endpoint() string {
	return fmt.Sprintf("%s:%d", s.Address, s.Port)
}

// ServiceDiscovery defines the interface for service discovery implementations
type ServiceDiscovery interface {
	// Discover returns available service instances
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Register registers a service instance (for dynamic discovery)
	Register(ctx context.Context, instance *ServiceInstance) error
	// Deregister removes a service instance
	Deregister(ctx context.Context, instanceID string) error
	// Watch returns a channel that emits updates when services change
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)
	// Close cleans up resources
	Close() error
}

// StaticDiscovery implements ServiceDiscovery with a static list of addresses
type StaticDiscovery struct {
	mu        sync.RWMutex
	instances map[string][]*ServiceInstance
}

// NewStaticDiscovery creates a new static discovery with the given addresses
func NewStaticDiscovery(services map[string][]string) *StaticDiscovery {
	sd := &StaticDiscovery{
		instances: make(map[string][]*ServiceInstance),
	}

	for name, addrs := range services {
		for i, addr := range addrs {
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
				portStr = "443"
			}
			port := 443
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				port = 443
			}

			sd.instances[name] = append(sd.instances[name], &ServiceInstance{
				ID:      fmt.Sprintf("%s-%d", name, i),
				Name:    name,
				Address: host,
				Port:    port,
				Healthy: true,
				Weight:  1,
			})
		}
	}

	return sd
}

// Discover returns all configured instances for a service
func (d *StaticDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	instances, ok := d.instances[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	// Return only healthy instances
	healthy := make([]*ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// Register adds a service instance (static discovery supports dynamic registration)
func (d *StaticDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.instances[instance.Name] = append(d.instances[instance.Name], instance)
	return nil
}

// Deregister removes a service instance
func (d *StaticDiscovery) Deregister(ctx context.Context, instanceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for name, instances := range d.instances {
		for i, inst := range instances {
			if inst.ID == instanceID {
				d.instances[name] = append(instances[:i], instances[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("instance %s not found", instanceID)
}

// Watch returns a channel for service updates (static discovery returns closed channel)
func (d *StaticDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	ch := make(chan []*ServiceInstance)
	close(ch) // Static discovery doesn't emit updates
	return ch, nil
}

// Close cleans up resources
func (d *StaticDiscovery) Close() error {
	return nil
}

// SetHealthy updates the health status of an instance
func (d *StaticDiscovery) SetHealthy(instanceID string, healthy bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, instances := range d.instances {
		for _, inst := range instances {
			if inst.ID == instanceID {
				inst.Healthy = healthy
				return
			}
		}
	}
}

// DNSDiscovery implements ServiceDiscovery using DNS SRV records
type DNSDiscovery struct {
	mu          sync.RWMutex
	resolver    *net.Resolver
	cache       map[string]*dnsCache
	cacheTTL    time.Duration
	defaultPort int
}

type dnsCache struct {
	instances []*ServiceInstance
	expiry    time.Time
}

// DNSDiscoveryConfig configures DNS-based discovery
type DNSDiscoveryConfig struct {
	CacheTTL    time.Duration
	DefaultPort int
	Resolver    *net.Resolver
}

// NewDNSDiscovery creates a new DNS-based service discovery
func NewDNSDiscovery(config DNSDiscoveryConfig) *DNSDiscovery {
	if config.CacheTTL == 0 {
		config.CacheTTL = 30 * time.Second
	}
	if config.DefaultPort == 0 {
		config.DefaultPort = 443
	}
	if config.Resolver == nil {
		config.Resolver = net.DefaultResolver
	}

	return &DNSDiscovery{
		resolver:    config.Resolver,
		cache:       make(map[string]*dnsCache),
		cacheTTL:    config.CacheTTL,
		defaultPort: config.DefaultPort,
	}
}

// Discover looks up SRV records for the service
func (d *DNSDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.mu.RLock()
	if cached, ok := d.cache[serviceName]; ok && time.Now().Before(cached.expiry) {
		d.mu.RUnlock()
		return cached.instances, nil
	}
	d.mu.RUnlock()

	// Try SRV lookup first
	_, srvs, err := d.resolver.LookupSRV(ctx, "", "", serviceName)
	if err == nil && len(srvs) > 0 {
		instances := make([]*ServiceInstance, 0, len(srvs))
		for i, srv := range srvs {
			instances = append(instances, &ServiceInstance{
				ID:      fmt.Sprintf("%s-%d", serviceName, i),
				Name:    serviceName,
				Address: srv.Target,
				Port:    int(srv.Port),
				Healthy: true,
				Weight:  int(srv.Weight),
			})
		}

		d.mu.Lock()
		d.cache[serviceName] = &dnsCache{
			instances: instances,
			expiry:    time.Now().Add(d.cacheTTL),
		}
		d.mu.Unlock()

		return instances, nil
	}

	// Fallback to A/AAAA lookup
	addrs, err := d.resolver.LookupHost(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %s: %w", serviceName, err)
	}

	instances := make([]*ServiceInstance, 0, len(addrs))
	for i, addr := range addrs {
		instances = append(instances, &ServiceInstance{
			ID:      fmt.Sprintf("%s-%d", serviceName, i),
			Name:    serviceName,
			Address: addr,
			Port:    d.defaultPort,
			Healthy: true,
			Weight:  1,
		})
	}

	d.mu.Lock()
	d.cache[serviceName] = &dnsCache{
		instances: instances,
		expiry:    time.Now().Add(d.cacheTTL),
	}
	d.mu.Unlock()

	return instances, nil
}

// Register is not supported for DNS discovery
func (d *DNSDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	return errors.New("DNS discovery does not support registration")
}

// Deregister is not supported for DNS discovery
func (d *DNSDiscovery) Deregister(ctx context.Context, instanceID string) error {
	return errors.New("DNS discovery does not support deregistration")
}

// Watch polls DNS at intervals for changes
func (d *DNSDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	ch := make(chan []*ServiceInstance, 1)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(d.cacheTTL)
		defer ticker.Stop()

		var lastInstances []*ServiceInstance

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				instances, err := d.Discover(ctx, serviceName)
				if err != nil {
					continue
				}

				if !instancesEqual(lastInstances, instances) {
					lastInstances = instances
					select {
					case ch <- instances:
					default:
					}
				}
			}
		}
	}()

	return ch, nil
}

// Close cleans up resources
func (d *DNSDiscovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*dnsCache)
	return nil
}

// KubernetesDiscovery implements ServiceDiscovery using Kubernetes endpoints API
type KubernetesDiscovery struct {
	mu        sync.RWMutex
	namespace string
	labelKey  string
	labelVal  string
	instances map[string][]*ServiceInstance
	watchers  map[string][]chan []*ServiceInstance
}

// KubernetesDiscoveryConfig configures Kubernetes-based discovery
type KubernetesDiscoveryConfig struct {
	Namespace  string
	LabelKey   string
	LabelValue string
}

// NewKubernetesDiscovery creates a new Kubernetes-based service discovery
func NewKubernetesDiscovery(config KubernetesDiscoveryConfig) *KubernetesDiscovery {
	if config.Namespace == "" {
		config.Namespace = "default"
	}

	return &KubernetesDiscovery{
		namespace: config.Namespace,
		labelKey:  config.LabelKey,
		labelVal:  config.LabelValue,
		instances: make(map[string][]*ServiceInstance),
		watchers:  make(map[string][]chan []*ServiceInstance),
	}
}

// Discover returns instances from Kubernetes endpoints
// Note: In production, this would use the Kubernetes client-go library
func (d *KubernetesDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	instances, ok := d.instances[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in namespace %s", serviceName, d.namespace)
	}

	healthy := make([]*ServiceInstance, 0)
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// Register adds an endpoint to the service
func (d *KubernetesDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}

	d.mu.Lock()
	d.instances[instance.Name] = append(d.instances[instance.Name], instance)
	watchers := d.watchers[instance.Name]
	instances := d.instances[instance.Name]
	d.mu.Unlock()

	// Notify watchers
	for _, ch := range watchers {
		select {
		case ch <- instances:
		default:
		}
	}

	return nil
}

// Deregister removes an endpoint
func (d *KubernetesDiscovery) Deregister(ctx context.Context, instanceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for name, instances := range d.instances {
		for i, inst := range instances {
			if inst.ID == instanceID {
				d.instances[name] = append(instances[:i], instances[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("instance %s not found", instanceID)
}

// Watch returns a channel for endpoint updates
func (d *KubernetesDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	ch := make(chan []*ServiceInstance, 1)

	d.mu.Lock()
	d.watchers[serviceName] = append(d.watchers[serviceName], ch)
	d.mu.Unlock()

	go func() {
		<-ctx.Done()
		d.mu.Lock()
		watchers := d.watchers[serviceName]
		for i, w := range watchers {
			if w == ch {
				d.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		d.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

// Close cleans up resources
func (d *KubernetesDiscovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, watchers := range d.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}
	d.watchers = make(map[string][]chan []*ServiceInstance)

	return nil
}

// UpdateEndpoints updates the endpoints for a service (simulates k8s watch)
func (d *KubernetesDiscovery) UpdateEndpoints(serviceName string, instances []*ServiceInstance) {
	d.mu.Lock()
	d.instances[serviceName] = instances
	watchers := d.watchers[serviceName]
	d.mu.Unlock()

	for _, ch := range watchers {
		select {
		case ch <- instances:
		default:
		}
	}
}

// ConsulDiscovery implements ServiceDiscovery using Consul
type ConsulDiscovery struct {
	mu        sync.RWMutex
	address   string
	token     string
	instances map[string][]*ServiceInstance
	watchers  map[string][]chan []*ServiceInstance
}

// ConsulDiscoveryConfig configures Consul-based discovery
type ConsulDiscoveryConfig struct {
	Address string
	Token   string
}

// NewConsulDiscovery creates a new Consul-based service discovery
func NewConsulDiscovery(config ConsulDiscoveryConfig) *ConsulDiscovery {
	if config.Address == "" {
		config.Address = "localhost:8500"
	}

	return &ConsulDiscovery{
		address:   config.Address,
		token:     config.Token,
		instances: make(map[string][]*ServiceInstance),
		watchers:  make(map[string][]chan []*ServiceInstance),
	}
}

// Discover queries Consul for healthy service instances
// Note: In production, this would use the Consul API client
func (d *ConsulDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	instances, ok := d.instances[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in Consul", serviceName)
	}

	healthy := make([]*ServiceInstance, 0)
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// Register registers a service with Consul
func (d *ConsulDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}

	d.mu.Lock()
	d.instances[instance.Name] = append(d.instances[instance.Name], instance)
	watchers := d.watchers[instance.Name]
	instances := d.instances[instance.Name]
	d.mu.Unlock()

	for _, ch := range watchers {
		select {
		case ch <- instances:
		default:
		}
	}

	return nil
}

// Deregister removes a service from Consul
func (d *ConsulDiscovery) Deregister(ctx context.Context, instanceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for name, instances := range d.instances {
		for i, inst := range instances {
			if inst.ID == instanceID {
				d.instances[name] = append(instances[:i], instances[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("instance %s not found", instanceID)
}

// Watch watches for service changes in Consul
func (d *ConsulDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	ch := make(chan []*ServiceInstance, 1)

	d.mu.Lock()
	d.watchers[serviceName] = append(d.watchers[serviceName], ch)
	d.mu.Unlock()

	go func() {
		<-ctx.Done()
		d.mu.Lock()
		watchers := d.watchers[serviceName]
		for i, w := range watchers {
			if w == ch {
				d.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		d.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

// Close cleans up resources
func (d *ConsulDiscovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, watchers := range d.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}
	d.watchers = make(map[string][]chan []*ServiceInstance)

	return nil
}

// UpdateService updates service instances (simulates Consul watch)
func (d *ConsulDiscovery) UpdateService(serviceName string, instances []*ServiceInstance) {
	d.mu.Lock()
	d.instances[serviceName] = instances
	watchers := d.watchers[serviceName]
	d.mu.Unlock()

	for _, ch := range watchers {
		select {
		case ch <- instances:
		default:
		}
	}
}

// Helper function to compare instance lists
func instancesEqual(a, b []*ServiceInstance) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]bool)
	for _, inst := range a {
		aMap[inst.Endpoint()] = true
	}

	for _, inst := range b {
		if !aMap[inst.Endpoint()] {
			return false
		}
	}

	return true
}

// Interface compliance
var (
	_ ServiceDiscovery = (*StaticDiscovery)(nil)
	_ ServiceDiscovery = (*DNSDiscovery)(nil)
	_ ServiceDiscovery = (*KubernetesDiscovery)(nil)
	_ ServiceDiscovery = (*ConsulDiscovery)(nil)
)
