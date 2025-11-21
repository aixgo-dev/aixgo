package agents

import (
	"context"
	"fmt"
	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/google/uuid"
	"log"
	"math/rand"
	"time"
)

type Producer struct{ def agent.AgentDef }

func init() {
	agent.Register("producer", func(d agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &Producer{def: d}, nil
	})
}

func (p *Producer) Start(ctx context.Context) error {
	rt, err := agent.RuntimeFromContext(ctx)
	if err != nil {
		return fmt.Errorf("runtime not found in context: %w", err)
	}

	t := time.NewTicker(p.def.Interval.Duration)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
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
}
