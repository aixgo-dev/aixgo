package agents

import (
	"context"
	"fmt"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"log"
)

type Logger struct {
	def agent.AgentDef
}

func init() {
	agent.Register("logger", func(d agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &Logger{def: d}, nil
	})
}

func (l *Logger) Name() string                                                      { return l.def.Name }
func (l *Logger) Role() string                                                      { return l.def.Role }
func (l *Logger) Ready() bool                                                       { return true }
func (l *Logger) Stop(ctx context.Context) error                                    { return nil }
func (l *Logger) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	if input != nil && input.Payload != "" {
		log.Printf("[ALERT] %s | %s", input.Type, input.Payload)
	}
	return input, nil
}

func (l *Logger) Start(ctx context.Context) error {
	rt, err := agent.RuntimeFromContext(ctx)
	if err != nil {
		return fmt.Errorf("runtime not found in context: %w", err)
	}

	for _, i := range l.def.Inputs {
		ch, err := rt.Recv(i.Source)
		if err != nil {
			return err
		}
		go func(s string, c <-chan *agent.Message) {
			for m := range c {
				log.Printf("[ALERT] %s | %s", m.Type, m.Payload)
			}
		}(i.Source, ch)
	}
	<-ctx.Done()
	return nil
}
