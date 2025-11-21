package mcp

import (
	"context"
	"testing"
	"time"
)

func TestStaticDiscovery(t *testing.T) {
	services := map[string][]string{
		"mcp-server": {"localhost:8080", "localhost:8081", "localhost:8082"},
	}

	discovery := NewStaticDiscovery(services)

	t.Run("Discover returns all healthy instances", func(t *testing.T) {
		instances, err := discovery.Discover(context.Background(), "mcp-server")
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if len(instances) != 3 {
			t.Errorf("expected 3 instances, got %d", len(instances))
		}

		for _, inst := range instances {
			if !inst.Healthy {
				t.Errorf("instance %s should be healthy", inst.ID)
			}
		}
	})

	t.Run("Discover returns error for unknown service", func(t *testing.T) {
		_, err := discovery.Discover(context.Background(), "unknown")
		if err == nil {
			t.Error("expected error for unknown service")
		}
	})

	t.Run("SetHealthy updates instance health", func(t *testing.T) {
		discovery.SetHealthy("mcp-server-0", false)

		instances, _ := discovery.Discover(context.Background(), "mcp-server")
		if len(instances) != 2 {
			t.Errorf("expected 2 healthy instances, got %d", len(instances))
		}

		// Restore health
		discovery.SetHealthy("mcp-server-0", true)
	})

	t.Run("Register adds new instance", func(t *testing.T) {
		err := discovery.Register(context.Background(), &ServiceInstance{
			ID:      "mcp-server-3",
			Name:    "mcp-server",
			Address: "localhost",
			Port:    8083,
			Healthy: true,
		})
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		instances, _ := discovery.Discover(context.Background(), "mcp-server")
		if len(instances) != 4 {
			t.Errorf("expected 4 instances after register, got %d", len(instances))
		}
	})

	t.Run("Deregister removes instance", func(t *testing.T) {
		err := discovery.Deregister(context.Background(), "mcp-server-3")
		if err != nil {
			t.Fatalf("Deregister failed: %v", err)
		}

		instances, _ := discovery.Discover(context.Background(), "mcp-server")
		if len(instances) != 3 {
			t.Errorf("expected 3 instances after deregister, got %d", len(instances))
		}
	})

	t.Run("Register with nil instance returns error", func(t *testing.T) {
		err := discovery.Register(context.Background(), nil)
		if err == nil {
			t.Error("expected error for nil instance")
		}
	})
}

func TestDNSDiscovery(t *testing.T) {
	config := DNSDiscoveryConfig{
		CacheTTL:    100 * time.Millisecond,
		DefaultPort: 8080,
	}
	discovery := NewDNSDiscovery(config)

	t.Run("Discover localhost", func(t *testing.T) {
		instances, err := discovery.Discover(context.Background(), "localhost")
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if len(instances) == 0 {
			t.Error("expected at least one instance")
		}

		for _, inst := range instances {
			if inst.Port != 8080 {
				t.Errorf("expected default port 8080, got %d", inst.Port)
			}
		}
	})

	t.Run("Register returns error", func(t *testing.T) {
		err := discovery.Register(context.Background(), &ServiceInstance{})
		if err == nil {
			t.Error("expected error for DNS register")
		}
	})

	t.Run("Deregister returns error", func(t *testing.T) {
		err := discovery.Deregister(context.Background(), "test")
		if err == nil {
			t.Error("expected error for DNS deregister")
		}
	})

	t.Run("Close clears cache", func(t *testing.T) {
		_, _ = discovery.Discover(context.Background(), "localhost")
		_ = discovery.Close()

		discovery.mu.RLock()
		cacheLen := len(discovery.cache)
		discovery.mu.RUnlock()

		if cacheLen != 0 {
			t.Errorf("expected empty cache after close, got %d entries", cacheLen)
		}
	})
}

func TestKubernetesDiscovery(t *testing.T) {
	config := KubernetesDiscoveryConfig{
		Namespace: "test-ns",
	}
	discovery := NewKubernetesDiscovery(config)

	t.Run("Register and Discover", func(t *testing.T) {
		_ = discovery.Register(context.Background(), &ServiceInstance{
			ID:      "pod-1",
			Name:    "my-service",
			Address: "10.0.0.1",
			Port:    8080,
			Healthy: true,
		})
		instances, err := discovery.Discover(context.Background(), "my-service")
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}
	})

	t.Run("Watch receives updates", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := discovery.Watch(ctx, "my-service")
		if err != nil {
			t.Fatalf("Watch failed: %v", err)
		}

		// Add a new instance
		_ = discovery.Register(context.Background(), &ServiceInstance{
			ID:      "pod-2",
			Name:    "my-service",
			Address: "10.0.0.2",
			Port:    8080,
			Healthy: true,
		})

		select {
		case instances := <-ch:
			if len(instances) != 2 {
				t.Errorf("expected 2 instances in watch update, got %d", len(instances))
			}
		case <-time.After(100 * time.Millisecond):
			// Watch notification is best-effort
		}
	})

	t.Run("UpdateEndpoints notifies watchers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, _ := discovery.Watch(ctx, "updated-service")

		newInstances := []*ServiceInstance{
			{ID: "new-1", Name: "updated-service", Address: "10.0.1.1", Port: 9090, Healthy: true},
			{ID: "new-2", Name: "updated-service", Address: "10.0.1.2", Port: 9090, Healthy: true},
		}

		discovery.UpdateEndpoints("updated-service", newInstances)

		select {
		case instances := <-ch:
			if len(instances) != 2 {
				t.Errorf("expected 2 instances, got %d", len(instances))
			}
		case <-time.After(100 * time.Millisecond):
			// Best-effort notification
		}
	})
}

func TestConsulDiscovery(t *testing.T) {
	config := ConsulDiscoveryConfig{
		Address: "localhost:8500",
	}
	discovery := NewConsulDiscovery(config)

	t.Run("Register and Discover", func(t *testing.T) {
		_ = discovery.Register(context.Background(), &ServiceInstance{
			ID:      "consul-1",
			Name:    "api-service",
			Address: "192.168.1.1",
			Port:    3000,
			Healthy: true,
		})
		instances, err := discovery.Discover(context.Background(), "api-service")
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		if len(instances) != 1 {
			t.Errorf("expected 1 instance, got %d", len(instances))
		}
	})

	t.Run("Deregister removes instance", func(t *testing.T) {
		_ = discovery.Register(context.Background(), &ServiceInstance{
			ID:      "consul-2",
			Name:    "api-service",
			Address: "192.168.1.2",
			Port:    3000,
			Healthy: true,
		})

		err := discovery.Deregister(context.Background(), "consul-2")
		if err != nil {
			t.Fatalf("Deregister failed: %v", err)
		}

		instances, _ := discovery.Discover(context.Background(), "api-service")
		if len(instances) != 1 {
			t.Errorf("expected 1 instance after deregister, got %d", len(instances))
		}
	})

	t.Run("UpdateService notifies watchers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, _ := discovery.Watch(ctx, "watched-service")

		newInstances := []*ServiceInstance{
			{ID: "w-1", Name: "watched-service", Address: "10.0.2.1", Port: 5000, Healthy: true},
		}

		discovery.UpdateService("watched-service", newInstances)

		select {
		case instances := <-ch:
			if len(instances) != 1 {
				t.Errorf("expected 1 instance, got %d", len(instances))
			}
		case <-time.After(100 * time.Millisecond):
			// Best-effort
		}
	})
}

func TestServiceInstanceEndpoint(t *testing.T) {
	inst := &ServiceInstance{
		Address: "example.com",
		Port:    8443,
	}

	endpoint := inst.Endpoint()
	expected := "example.com:8443"

	if endpoint != expected {
		t.Errorf("expected %s, got %s", expected, endpoint)
	}
}

func TestInstancesEqual(t *testing.T) {
	a := []*ServiceInstance{
		{Address: "host1", Port: 80},
		{Address: "host2", Port: 80},
	}

	b := []*ServiceInstance{
		{Address: "host2", Port: 80},
		{Address: "host1", Port: 80},
	}

	c := []*ServiceInstance{
		{Address: "host1", Port: 80},
	}

	if !instancesEqual(a, b) {
		t.Error("a and b should be equal")
	}

	if instancesEqual(a, c) {
		t.Error("a and c should not be equal")
	}

	if instancesEqual(nil, a) {
		t.Error("nil and a should not be equal")
	}

	if !instancesEqual(nil, nil) {
		t.Error("nil and nil should be equal")
	}
}
