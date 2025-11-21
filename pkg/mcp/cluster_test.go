package mcp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterCreation(t *testing.T) {
	t.Run("creates cluster with valid config", func(t *testing.T) {
		discovery := NewStaticDiscovery(map[string][]string{
			"test-service": {"localhost:8080"},
		})

		cluster, err := NewCluster(ClusterConfig{
			ServiceName: "test-service",
			Discovery:   discovery,
		})

		if err != nil {
			t.Fatalf("NewCluster failed: %v", err)
		}

		if cluster == nil {
			t.Fatal("cluster should not be nil")
		}
	})

	t.Run("fails without discovery", func(t *testing.T) {
		_, err := NewCluster(ClusterConfig{
			ServiceName: "test-service",
		})

		if err == nil {
			t.Error("expected error without discovery")
		}
	})

	t.Run("fails without service name", func(t *testing.T) {
		discovery := NewStaticDiscovery(map[string][]string{})
		_, err := NewCluster(ClusterConfig{
			Discovery: discovery,
		})

		if err == nil {
			t.Error("expected error without service name")
		}
	})

	t.Run("applies default values", func(t *testing.T) {
		discovery := NewStaticDiscovery(map[string][]string{
			"test": {"localhost:8080"},
		})

		cluster, _ := NewCluster(ClusterConfig{
			ServiceName: "test",
			Discovery:   discovery,
		})

		if cluster.config.Strategy != RoundRobin {
			t.Error("default strategy should be RoundRobin")
		}

		if cluster.config.HealthCheckInterval != 10*time.Second {
			t.Error("default health check interval should be 10s")
		}

		if cluster.config.MaxFailures != 3 {
			t.Error("default max failures should be 3")
		}
	})
}

func TestClusterLoadBalancing(t *testing.T) {
	services := map[string][]string{
		"lb-service": {"localhost:8080", "localhost:8081", "localhost:8082"},
	}
	discovery := NewStaticDiscovery(services)

	t.Run("round robin distributes evenly", func(t *testing.T) {
		cluster, _ := NewCluster(ClusterConfig{
			ServiceName: "lb-service",
			Discovery:   discovery,
			Strategy:    RoundRobin,
		})

		ctx := context.Background()
		_ = cluster.Start(ctx)
		defer func() {
			_ = cluster.Stop()
		}()

		counts := make(map[string]int)
		for i := 0; i < 9; i++ {
			node, err := cluster.GetNode()
			if err != nil {
				t.Fatalf("GetNode failed: %v", err)
			}
			counts[node.Instance.ID]++
		}

		for _, count := range counts {
			if count != 3 {
				t.Errorf("expected 3 requests per node, got distribution: %v", counts)
				break
			}
		}
	})

	t.Run("least connections selects node with fewest connections", func(t *testing.T) {
		cluster, _ := NewCluster(ClusterConfig{
			ServiceName: "lb-service",
			Discovery:   discovery,
			Strategy:    LeastConnections,
		})

		ctx := context.Background()
		_ = cluster.Start(ctx)
		defer func() {
			_ = cluster.Stop()
		}()

		nodes := cluster.Nodes()
		if len(nodes) < 2 {
			t.Skip("need at least 2 nodes")
		}

		// Simulate connections - one node has many, others have few
		atomic.StoreInt64(&nodes[0].Connections, 100)
		for i := 1; i < len(nodes); i++ {
			atomic.StoreInt64(&nodes[i].Connections, 0)
		}

		node, _ := cluster.GetNode()
		// Should not select the heavily loaded node
		if node.GetConnections() == 100 {
			t.Error("should not select node with most connections")
		}
	})

	t.Run("random selects from healthy nodes", func(t *testing.T) {
		cluster, _ := NewCluster(ClusterConfig{
			ServiceName: "lb-service",
			Discovery:   discovery,
			Strategy:    Random,
		})

		ctx := context.Background()
		_ = cluster.Start(ctx)
		defer func() {
			_ = cluster.Stop()
		}()

		for i := 0; i < 10; i++ {
			node, err := cluster.GetNode()
			if err != nil {
				t.Fatalf("GetNode failed: %v", err)
			}
			if node.State != NodeStateHealthy {
				t.Error("should only select healthy nodes")
			}
		}
	})

	t.Run("weighted random respects weights", func(t *testing.T) {
		weightedDiscovery := NewStaticDiscovery(map[string][]string{})
		_ = weightedDiscovery.Register(context.Background(), &ServiceInstance{
			ID: "heavy", Name: "weighted-svc", Address: "localhost", Port: 8080, Healthy: true, Weight: 100,
		})
		_ = weightedDiscovery.Register(context.Background(), &ServiceInstance{
			ID: "light", Name: "weighted-svc", Address: "localhost", Port: 8081, Healthy: true, Weight: 1,
		})

		cluster, _ := NewCluster(ClusterConfig{
			ServiceName: "weighted-svc",
			Discovery:   weightedDiscovery,
			Strategy:    WeightedRandom,
		})

		ctx := context.Background()
		_ = cluster.Start(ctx)
		defer func() {
			_ = cluster.Stop()
		}()

		counts := make(map[string]int)
		for i := 0; i < 1000; i++ {
			node, _ := cluster.GetNode()
			counts[node.Instance.ID]++
		}

		// Heavy should get significantly more
		if counts["heavy"] < counts["light"]*10 {
			t.Errorf("weighted distribution not working: heavy=%d, light=%d", counts["heavy"], counts["light"])
		}
	})
}

func TestClusterNodeManagement(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"mgmt-service": {"localhost:8080"},
	})

	cluster, _ := NewCluster(ClusterConfig{
		ServiceName: "mgmt-service",
		Discovery:   discovery,
	})

	ctx := context.Background()
	_ = cluster.Start(ctx)
	defer func() {
		_ = cluster.Stop()
	}()

	t.Run("RegisterNode adds node", func(t *testing.T) {
		initialCount := len(cluster.Nodes())

		err := cluster.RegisterNode(&ServiceInstance{
			ID:      "manual-node",
			Name:    "mgmt-service",
			Address: "localhost",
			Port:    9000,
			Healthy: true,
		})

		if err != nil {
			t.Fatalf("RegisterNode failed: %v", err)
		}

		if len(cluster.Nodes()) != initialCount+1 {
			t.Error("node count should increase after register")
		}
	})

	t.Run("RegisterNode is idempotent", func(t *testing.T) {
		count := len(cluster.Nodes())

		_ = cluster.RegisterNode(&ServiceInstance{
			ID:      "manual-node",
			Name:    "mgmt-service",
			Address: "localhost",
			Port:    9000,
			Healthy: true,
		})

		if len(cluster.Nodes()) != count {
			t.Error("duplicate register should not add node")
		}
	})

	t.Run("DeregisterNode removes node", func(t *testing.T) {
		initialCount := len(cluster.Nodes())

		err := cluster.DeregisterNode("manual-node")
		if err != nil {
			t.Fatalf("DeregisterNode failed: %v", err)
		}

		if len(cluster.Nodes()) != initialCount-1 {
			t.Error("node count should decrease after deregister")
		}
	})

	t.Run("DeregisterNode returns error for unknown node", func(t *testing.T) {
		err := cluster.DeregisterNode("unknown-node")
		if err == nil {
			t.Error("expected error for unknown node")
		}
	})
}

func TestClusterHealthyNodes(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"health-service": {"localhost:8080", "localhost:8081"},
	})

	cluster, _ := NewCluster(ClusterConfig{
		ServiceName: "health-service",
		Discovery:   discovery,
	})

	ctx := context.Background()
	_ = cluster.Start(ctx)
	defer func() {
		_ = cluster.Stop()
	}()

	t.Run("returns only healthy nodes", func(t *testing.T) {
		healthy := cluster.HealthyNodes()
		if len(healthy) != 2 {
			t.Errorf("expected 2 healthy nodes, got %d", len(healthy))
		}

		// Mark one unhealthy
		nodes := cluster.Nodes()
		nodes[0].State = NodeStateUnhealthy

		healthy = cluster.HealthyNodes()
		if len(healthy) != 1 {
			t.Errorf("expected 1 healthy node, got %d", len(healthy))
		}
	})

	t.Run("GetNode returns error when no healthy nodes", func(t *testing.T) {
		nodes := cluster.Nodes()
		for _, n := range nodes {
			n.State = NodeStateUnhealthy
		}

		_, err := cluster.GetNode()
		if err == nil {
			t.Error("expected error when no healthy nodes")
		}
	})
}

func TestClusterNodeConnections(t *testing.T) {
	node := &ClusterNode{
		Instance: &ServiceInstance{ID: "test"},
	}

	t.Run("increment and decrement connections", func(t *testing.T) {
		node.IncrementConnections()
		node.IncrementConnections()

		if node.GetConnections() != 2 {
			t.Errorf("expected 2 connections, got %d", node.GetConnections())
		}

		node.DecrementConnections()

		if node.GetConnections() != 1 {
			t.Errorf("expected 1 connection, got %d", node.GetConnections())
		}
	})
}

func TestClusterTransport(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"transport-service": {"localhost:8080"},
	})

	cluster, _ := NewCluster(ClusterConfig{
		ServiceName: "transport-service",
		Discovery:   discovery,
	})

	transport := NewClusterTransport(cluster)

	t.Run("implements Transport interface", func(t *testing.T) {
		var _ Transport = transport
	})

	t.Run("Close stops cluster", func(t *testing.T) {
		ctx := context.Background()
		_ = cluster.Start(ctx)

		err := transport.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
}

func TestClusterStartStop(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"lifecycle-service": {"localhost:8080"},
	})

	cluster, _ := NewCluster(ClusterConfig{
		ServiceName: "lifecycle-service",
		Discovery:   discovery,
	})

	t.Run("Start is idempotent", func(t *testing.T) {
		ctx := context.Background()

		err := cluster.Start(ctx)
		if err != nil {
			t.Fatalf("first Start failed: %v", err)
		}

		err = cluster.Start(ctx)
		if err != nil {
			t.Fatalf("second Start failed: %v", err)
		}
	})

	t.Run("Stop is idempotent", func(t *testing.T) {
		err := cluster.Stop()
		if err != nil {
			t.Fatalf("first Stop failed: %v", err)
		}

		err = cluster.Stop()
		if err != nil {
			t.Fatalf("second Stop failed: %v", err)
		}
	})
}

func TestClusterFailureHandling(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"failure-service": {"localhost:8080"},
	})

	cluster, _ := NewCluster(ClusterConfig{
		ServiceName: "failure-service",
		Discovery:   discovery,
		MaxFailures: 2,
	})

	ctx := context.Background()
	_ = cluster.Start(ctx)
	defer func() {
		_ = cluster.Stop()
	}()

	t.Run("marks node unhealthy after max failures", func(t *testing.T) {
		nodes := cluster.Nodes()
		node := nodes[0]

		// Simulate failures
		cluster.handleNodeFailure(node)
		if node.State != NodeStateHealthy {
			t.Error("should still be healthy after 1 failure")
		}

		cluster.handleNodeFailure(node)
		if node.State != NodeStateUnhealthy {
			t.Error("should be unhealthy after max failures")
		}
	})
}
