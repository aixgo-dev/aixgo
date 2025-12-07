package agents

import (
	"context"
	"fmt"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/google/uuid"
	"log"
	"math/rand"
	"time"
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
				e := 100 + rand.Float64()*900
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
