package agents

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/google/uuid"
	pb "github.com/aixgo-dev/aixgo/proto"
)

type Producer struct{ def agent.AgentDef }
func init() {
	agent.Register("producer", func(d agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &Producer{def: d}, nil
	})
}

func (p *Producer) Start(ctx context.Context) error {
	rt, ok := agent.RuntimeFromContext(ctx)
	if !ok {
		return fmt.Errorf("runtime not found in context")
	}

	t := time.NewTicker(p.def.Interval.Duration)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			e := 100 + rand.Float64()*900
			m := &agent.Message{&pb.Message{
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
