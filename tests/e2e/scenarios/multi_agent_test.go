package scenarios

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/aixgo-dev/aixgo/tests/e2e"
)

// TestMultiAgent_BasicCoordination tests basic multi-agent coordination
func TestMultiAgent_BasicCoordination(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Define two agents
	agent1Def := e2e.CreateTestAgentDef("analyst", "react", "gpt-4")
	agent2Def := e2e.CreateTestAgentDef("writer", "react", "gpt-4")

	// Setup message routing: analyst -> writer
	agent1Def.Outputs = []agent.Output{{Target: "input-writer"}}
	agent2Def.Inputs = []agent.Input{{Source: "input-writer"}}

	// Setup provider responses
	env.Provider().AddTextResponse("Analysis complete: The data shows positive trends.")
	env.Provider().AddTextResponse("Based on the analysis, here is the final report.")

	// Simulate agent 1 processing
	resp1, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: agent1Def.Prompt},
			{Role: "user", Content: "Analyze this data"},
		},
	})
	e2e.AssertNoError(t, err, "agent 1 processing")

	// Route to agent 2
	msg := &agent.Message{
		Message: &pb.Message{
			Id:        "msg-1",
			Type:      "analysis",
			Payload:   resp1.Content,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	err = env.Runtime().Send(agent1Def.Outputs[0].Target, msg)
	e2e.AssertNoError(t, err, "routing to agent 2")

	// Simulate agent 2 processing
	resp2, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: agent2Def.Prompt},
			{Role: "user", Content: resp1.Content},
		},
	})
	e2e.AssertNoError(t, err, "agent 2 processing")

	e2e.AssertContains(t, resp2.Content, "report", "agent 2 should produce report")

	// Verify routing
	calls := env.Runtime().GetSendCalls()
	e2e.AssertEqual(t, 1, len(calls), "should have one send call")
	e2e.AssertEqual(t, "input-writer", calls[0].Target, "target should be writer input")
}

// TestMultiAgent_ParallelProcessing tests parallel agent processing
func TestMultiAgent_ParallelProcessing(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Add responses for parallel agents
	for i := 0; i < 3; i++ {
		env.Provider().AddTextResponse("Processed successfully")
	}

	agents := []agent.AgentDef{
		e2e.CreateTestAgentDef("agent-1", "react", "gpt-4"),
		e2e.CreateTestAgentDef("agent-2", "react", "gpt-4"),
		e2e.CreateTestAgentDef("agent-3", "react", "gpt-4"),
	}

	var wg sync.WaitGroup
	results := make(chan string, len(agents))

	for _, def := range agents {
		wg.Add(1)
		go func(d agent.AgentDef) {
			defer wg.Done()

			resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "system", Content: d.Prompt},
					{Role: "user", Content: "Process this task"},
				},
			})
			if err != nil {
				t.Errorf("agent %s failed: %v", d.Name, err)
				return
			}
			results <- resp.Content
		}(def)
	}

	wg.Wait()
	close(results)

	count := 0
	for range results {
		count++
	}

	e2e.AssertEqual(t, len(agents), count, "all agents should complete")
}

// TestMultiAgent_ChainedProcessing tests chained agent processing
func TestMultiAgent_ChainedProcessing(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup responses for chain
	env.Provider().AddTextResponse("Step 1: Data extracted")
	env.Provider().AddTextResponse("Step 2: Data transformed")
	env.Provider().AddTextResponse("Step 3: Data loaded")

	chain := []struct {
		name   string
		input  string
		output string
	}{
		{name: "extractor", input: "source", output: "transformer-input"},
		{name: "transformer", input: "transformer-input", output: "loader-input"},
		{name: "loader", input: "loader-input", output: "sink"},
	}

	currentPayload := "Initial data"

	for i, step := range chain {
		resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: currentPayload},
			},
		})
		e2e.AssertNoError(t, err, "step "+step.name)

		// Route to next step
		msg := &agent.Message{
			Message: &pb.Message{
				Id:      "chain-" + string(rune('0'+i)),
				Payload: resp.Content,
			},
		}
		err = env.Runtime().Send(step.output, msg)
		e2e.AssertNoError(t, err, "routing from "+step.name)

		currentPayload = resp.Content
	}

	// Verify chain completed
	calls := env.Runtime().GetSendCalls()
	e2e.AssertEqual(t, len(chain), len(calls), "all chain steps should route")
}

// TestMultiAgent_ErrorPropagation tests error propagation between agents
func TestMultiAgent_ErrorPropagation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup: first agent succeeds, second detects error
	env.Provider().AddTextResponse("Processing complete: ERROR_DETECTED")
	env.Provider().AddTextResponse("Error handling: Retrying operation")

	// Agent 1 processes and returns error indicator
	resp1, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Process data"},
		},
	})
	e2e.AssertNoError(t, err, "agent 1")

	// Check if error was detected
	hasError := false
	for i := 0; i <= len(resp1.Content)-14; i++ {
		if resp1.Content[i:i+14] == "ERROR_DETECTED" {
			hasError = true
			break
		}
	}

	if hasError {
		// Agent 2 handles the error
		resp2, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Handle error: " + resp1.Content},
			},
		})
		e2e.AssertNoError(t, err, "error handling agent")
		e2e.AssertContains(t, resp2.Content, "Retry", "should indicate retry")
	}
}

// TestMultiAgent_MessageAggregation tests aggregating messages from multiple agents
func TestMultiAgent_MessageAggregation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup responses from multiple worker agents
	env.Provider().AddTextResponse("Result from worker 1: Data A")
	env.Provider().AddTextResponse("Result from worker 2: Data B")
	env.Provider().AddTextResponse("Result from worker 3: Data C")
	env.Provider().AddTextResponse("Aggregated result: Data A + Data B + Data C")

	// Simulate worker agents
	workerResults := make([]string, 3)

	for i := 0; i < 3; i++ {
		resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Process part " + string(rune('A'+i))},
			},
		})
		e2e.AssertNoError(t, err, "worker "+string(rune('0'+i)))
		workerResults[i] = resp.Content
	}

	// Aggregator combines results
	combinedInput := ""
	for _, r := range workerResults {
		combinedInput += r + "\n"
	}

	aggResp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Aggregate: " + combinedInput},
		},
	})
	e2e.AssertNoError(t, err, "aggregator")
	e2e.AssertContains(t, aggResp.Content, "Aggregated", "should show aggregation")
}

// TestMultiAgent_Timeout tests timeout handling in multi-agent scenarios
func TestMultiAgent_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Simulate slow operation
	done := make(chan bool, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done <- true
	}()

	select {
	case <-done:
		// Success - operation completed in time
	case <-ctx.Done():
		t.Error("operation should complete before timeout")
	}

	// Test actual timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	done2 := make(chan bool, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done2 <- true
	}()

	select {
	case <-done2:
		t.Error("operation should have timed out")
	case <-ctx2.Done():
		// Expected timeout
	}
}

// TestMultiAgent_FanOutFanIn tests fan-out/fan-in pattern
func TestMultiAgent_FanOutFanIn(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Add responses for fan-out workers
	for i := 0; i < 5; i++ {
		env.Provider().AddTextResponse("Processed chunk " + string(rune('A'+i)))
	}

	// Fan-out: distribute work
	numWorkers := 5
	results := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Process chunk " + string(rune('A'+workerID))},
				},
			})
			if err != nil {
				return
			}
			results <- resp.Content
		}(i)
	}

	wg.Wait()
	close(results)

	// Fan-in: collect results
	var collected []string
	for r := range results {
		collected = append(collected, r)
	}

	e2e.AssertEqual(t, numWorkers, len(collected), "all workers should complete")
}
