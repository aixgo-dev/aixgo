package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/sashabaranov/go-openai"
)

// OpenAIClient interface for testability
type OpenAIClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type ReActAgent struct {
	def    agent.AgentDef
	client OpenAIClient
	model  string
	tools  map[string]func(context.Context, map[string]any) (any, error)
	rt     agent.Runtime
}

func init() {
	agent.Register("react", NewReActAgent)
}

// NewReActAgent creates a new ReActAgent with default OpenAI client
func NewReActAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
	client := openai.NewClient("xai-api-key-placeholder")
	return NewReActAgentWithClient(def, rt, client)
}

// NewReActAgentWithClient creates a new ReActAgent with a custom client (useful for testing)
func NewReActAgentWithClient(def agent.AgentDef, rt agent.Runtime, client OpenAIClient) (agent.Agent, error) {
	tools := make(map[string]func(context.Context, map[string]any) (any, error))
	for _, t := range def.Tools {
		validator := llm.NewValidator(t.InputSchema)
		toolName := t.Name
		tools[toolName] = func(ctx context.Context, in map[string]any) (any, error) {
			if err := validator.Validate(in); err != nil {
				return nil, err
			}
			return map[string]string{"status": "ok"}, nil
		}
	}
	return &ReActAgent{
		def:    def,
		client: client,
		model:  def.Model,
		tools:  tools,
		rt:     rt,
	}, nil
}

func (r *ReActAgent) Start(ctx context.Context) error {
	if len(r.def.Inputs) == 0 {
		return fmt.Errorf("no inputs defined for ReActAgent")
	}

	ch, err := r.rt.Recv(r.def.Inputs[0].Source)
	if err != nil {
		return fmt.Errorf("failed to receive from %s: %w", r.def.Inputs[0].Source, err)
	}

	for m := range ch {
		span := observability.StartSpan("react.think", map[string]any{"input": m.Payload})
		res, err := r.think(ctx, m.Payload)
		span.End()
		if err != nil {
			log.Printf("ReAct error: %v", err)
			continue
		}
		out := &agent.Message{&pb.Message{
			Id:        m.Id,
			Type:      "analysis",
			Payload:   res,
			Timestamp: time.Now().Format(time.RFC3339),
		}}
		for _, o := range r.def.Outputs {
			if err := r.rt.Send(o.Target, out); err != nil {
				log.Printf("Error sending to %s: %v", o.Target, err)
			}
		}
	}
	return nil
}

func (r *ReActAgent) think(ctx context.Context, input string) (string, error) {
	tools := make([]openai.Tool, len(r.def.Tools))
	for i, t := range r.def.Tools {
		tools[i] = openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(mustMarshal(t.InputSchema)),
			},
		}
	}

	req := openai.ChatCompletionRequest{
		Model: r.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: r.def.Prompt},
			{Role: "user", Content: input},
		},
		Tools: tools,
	}

	resp, err := r.client.CreateChatCompletion(ctx, req)
	if err != nil { return "", err }

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		call := resp.Choices[0].Message.ToolCalls[0]
		var args map[string]any
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		toolFunc, ok := r.tools[call.Function.Name]
		if !ok {
			return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
		}

		result, err := toolFunc(ctx, args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Tool %s → %v", call.Function.Name, result), nil
	}
	return resp.Choices[0].Message.Content, nil
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// This should never happen with valid schema, but handle it gracefully
		log.Printf("Warning: failed to marshal value: %v", err)
		return []byte("{}")
	}
	return b
}
