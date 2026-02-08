package agents

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/google/uuid"
)

type Producer struct {
	def    agent.AgentDef
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func init() {
	agent.Register("producer", func(d agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &Producer{def: d}, nil
	})
}

func (p *Producer) Name() string  { return p.def.Name }
func (p *Producer) Role() string  { return p.def.Role }
func (p *Producer) Ready() bool   { return true }
func (p *Producer) Stop(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}
func (p *Producer) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// Producer doesn't support sync execution - it's designed for async event generation
	return nil, &agent.NotImplementedError{AgentName: p.def.Name, Method: "Execute"}
}

func (p *Producer) Start(ctx context.Context) error {
	rt, err := agent.RuntimeFromContext(ctx)
	if err != nil {
		return fmt.Errorf("runtime not found in context: %w", err)
	}

	// Create cancellable context
	ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		t := time.NewTicker(p.def.Interval.Duration)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// G404: Use crypto/rand for generating random values
				e := 100 + cryptoRandFloat64()*900
				m := &agent.Message{Message: &pb.Message{
					Id:        uuid.NewString(),
					Type:      "ray_burst",
					Payload:   fmt.Sprintf("Cosmic ray: %.1f TeV", e),
					Timestamp: time.Now().Format(time.RFC3339),
				}}
				for _, o := range p.def.Outputs {
					if err := rt.Send(o.Target, m); err != nil {
						log.Printf("Error sending to %s: %v", o.Target, err)
					}
				}
				log.Printf("Detected %.1f TeV", e)
			}
		}
	}()

	<-ctx.Done()
	return nil
}

// cryptoRandFloat64 returns a cryptographically secure random float64 in [0.0, 1.0)
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to deterministic value on error (should never happen)
		return 0.5
	}
	// Use top 53 bits to create a float64 in [0, 1)
	return float64(binary.BigEndian.Uint64(b[:])>>11) / (1 << 53)
}
