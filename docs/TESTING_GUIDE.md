# Testing Guide

This guide outlines the testing strategy, available test utilities, and best practices for testing aixgo components.

**For comprehensive documentation, visit [https://aixgo.dev](https://aixgo.dev)**

## Table of Contents

- [Testing Philosophy](#testing-philosophy)
- [Test Utilities](#test-utilities)
- [Testing Strategies by Component](#testing-strategies-by-component)
- [Coverage Goals](#coverage-goals)
- [Running Tests](#running-tests)
- [Writing Tests](#writing-tests)

## Testing Philosophy

Aixgo follows these testing principles:

- **High Coverage**: Aim for >80% overall, 100% for critical paths
- **Testability First**: All components designed for easy testing
- **No External Dependencies**: Use mocks for LLM APIs, file systems, and network calls
- **Thread Safety**: Always test with race detector
- **Isolation**: Tests should not share state or depend on execution order

## Test Utilities

Aixgo provides comprehensive mock implementations for testing.

### MockRuntime

Mock implementation of the runtime for testing agent communication.

```go
import "github.com/aixgo-dev/aixgo/internal/agent"

// Create mock runtime
mockRT := agent.NewMockRuntime()

// Create context with runtime
ctx := agent.ContextWithRuntime(context.Background(), mockRT)

// Inject errors for testing error paths
mockRT.SetSendError(errors.New("channel full"))

// Simulate incoming messages
mockRT.SendMessage("test-channel", &proto.Message{
    Content: "test message",
})

// Verify agent behavior
calls := mockRT.GetSendCalls()
if len(calls) != expectedCount {
    t.Errorf("expected %d sends, got %d", expectedCount, len(calls))
}
```

**Features:**

- Records all Send() and Recv() calls
- Supports error injection
- Non-blocking operations to prevent test hangs
- Thread-safe for parallel tests

### MockOpenAIClient

Mock OpenAI client for testing ReAct agents without API calls.

```go
import "github.com/aixgo-dev/aixgo/agents"

// Create mock client
mockClient := agents.NewMockOpenAIClient()

// Configure responses
mockClient.AddResponse(openai.ChatCompletionResponse{
    Choices: []openai.ChatCompletionChoice{
        {
            Message: openai.ChatCompletionMessage{
                Content: "Test response",
            },
        },
    },
}, nil)

// Inject errors
mockClient.AddResponse(openai.ChatCompletionResponse{}, errors.New("API error"))

// Use in tests
agent := agents.NewReActAgentWithClient(def, mockClient)

// Verify API calls
calls := mockClient.GetCalls()
```

**Features:**

- Configurable responses and errors
- Call recording for verification
- Thread-safe operations
- Reset() method for test cleanup

### MockFileReader

Mock file system for testing configuration loading.

```go
import "github.com/aixgo-dev/aixgo"

// Create mock file reader
mockReader := aixgo.NewMockFileReader()

// Add test files
mockReader.AddFile("config/agents.yaml", []byte(`
supervisor:
  name: test-supervisor
agents:
  - name: test-agent
    role: producer
`))

// Inject errors
mockReader.SetError("missing.yaml", errors.New("file not found"))

// Use in tests
loader := aixgo.NewConfigLoader(mockReader)
config, err := loader.Load("config/agents.yaml")
```

**Features:**

- In-memory file system
- Error injection support
- Thread-safe operations

### Test Helpers

Convenience functions for creating test fixtures:

```go
// Create context with runtime
ctx := agent.ContextWithRuntime(context.Background(), mockRuntime)

// Create test agent definition
def := agent.TestAgentDef(
    agent.WithName("test-agent"),
    agent.WithRole("react"),
)
```

## Testing Strategies by Component

### Agent Registry

**File:** `internal/agent/types_test.go`

**Strategy:**

- Test isolated registry instances (no global state)
- Test concurrent registration and retrieval
- Test unknown role handling

**Example:**

```go
func TestRegistry(t *testing.T) {
    t.Parallel()

    registry := agent.NewRegistry()

    // Register factory
    called := false
    registry.Register("custom", func(def agent.AgentDef) agent.Agent {
        called = true
        return &customAgent{}
    })

    // Retrieve and use factory
    factory, exists := registry.Get("custom")
    if !exists {
        t.Fatal("expected factory to exist")
    }

    agent := factory(agent.TestAgentDef())
    if !called {
        t.Error("factory not called")
    }
}
```

### Runtime

**File:** `runtime_test.go`

**Strategy:**

- Test concurrent Send/Recv operations
- Test channel creation and auto-initialization
- Test channel buffer overflow scenarios
- Test with race detector

**Example:**

```go
func TestRuntimeConcurrency(t *testing.T) {
    t.Parallel()

    rt := aixgo.NewRuntime()

    // Launch concurrent senders
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            msg := &proto.Message{Content: fmt.Sprintf("msg-%d", id)}
            rt.Send("test", msg)
        }(i)
    }

    wg.Wait()

    // Verify all messages received
    received := 0
    timeout := time.After(5 * time.Second)
    for received < 100 {
        select {
        case msg := <-rt.Recv("test"):
            if msg != nil {
                received++
            }
        case <-timeout:
            t.Fatalf("timeout: received %d/100 messages", received)
        }
    }
}
```

### LLM Validation

**File:** `internal/llm/validation_test.go`

**Strategy:**

- Use table-driven tests for type validation
- Test all type branches (string, number, boolean, object, array)
- Test required field validation
- Test numeric constraints (minimum, maximum)
- Test edge cases (nil values, empty schemas, unknown fields)

**Example:**

```go
func TestValidation(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        schema  map[string]interface{}
        args    map[string]interface{}
        wantErr bool
    }{
        {
            name: "valid string",
            schema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "name": map[string]interface{}{"type": "string"},
                },
                "required": []interface{}{"name"},
            },
            args:    map[string]interface{}{"name": "test"},
            wantErr: false,
        },
        // More cases...
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            validator := llm.NewValidator(tt.schema)
            err := validator.Validate(tt.args)

            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Agent Implementations

**Files:** `agents/*_test.go`

**Strategy:**

- Test with mock runtime and clients
- Test message generation/consumption
- Test error handling (missing inputs, send failures, context cancellation)
- Test configuration validation

**Example:**

```go
func TestReActAgent(t *testing.T) {
    t.Parallel()

    mockClient := agents.NewMockOpenAIClient()
    mockClient.AddResponse(openai.ChatCompletionResponse{
        Choices: []openai.ChatCompletionChoice{
            {Message: openai.ChatCompletionMessage{Content: "Response"}},
        },
    }, nil)

    mockRT := agent.NewMockRuntime()
    ctx := agent.ContextWithRuntime(context.Background(), mockRT)

    def := agent.TestAgentDef(
        agent.WithName("test-agent"),
        agent.WithRole("react"),
        agent.WithInputs([]string{"input-channel"}),
    )

    agent := agents.NewReActAgentWithClient(def, mockClient)

    // Start agent
    go agent.Start(ctx)

    // Send test message
    mockRT.SendMessage("input-channel", &proto.Message{
        Content: "test query",
    })

    // Verify agent processed message
    time.Sleep(100 * time.Millisecond)

    calls := mockClient.GetCalls()
    if len(calls) != 1 {
        t.Errorf("expected 1 API call, got %d", len(calls))
    }
}
```

### Config Loading

**File:** `aixgo_test.go`

**Strategy:**

- Test with mock file reader
- Test YAML parsing (valid and invalid)
- Test file read errors
- Test agent creation from config

**Example:**

```go
func TestConfigLoading(t *testing.T) {
    t.Parallel()

    mockReader := aixgo.NewMockFileReader()
    mockReader.AddFile("config.yaml", []byte(`
supervisor:
  name: test-supervisor
  model: gpt-4-turbo
agents:
  - name: producer
    role: producer
    interval: 1s
`))

    loader := aixgo.NewConfigLoader(mockReader)
    config, err := loader.Load("config.yaml")

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if config.Supervisor.Name != "test-supervisor" {
        t.Errorf("expected supervisor name 'test-supervisor', got '%s'",
            config.Supervisor.Name)
    }

    if len(config.Agents) != 1 {
        t.Errorf("expected 1 agent, got %d", len(config.Agents))
    }
}
```

## Coverage Goals

### Critical Components (100% Coverage Required)

- Agent Factory (`internal/agent/factory.go`)
- Agent Registry (`internal/agent/types.go`)
- Runtime (`runtime.go`)
- Validation (`internal/llm/validation.go`)

### Standard Components (>80% Coverage)

- Agent Implementations (`agents/*.go`)
- Config Loading (`aixgo.go`)
- Supervisor (`internal/supervisor/*.go`)

### Supporting Components (>60% Coverage)

- Observability (`internal/observability/*.go`)
- Protocol (`proto/*.go`)

### Excluded from Coverage

- Test utilities (`*testutil.go`)
- Example applications (`examples/`)
- Generated code

## Running Tests

### Basic Test Execution

```bash
# Run all tests
go test ./...

# Run specific package
go test ./internal/agent

# Run specific test
go test -run TestAgentRegistry ./internal/agent

# Verbose output
go test -v ./...
```

### Race Detection

Always test for race conditions:

```bash
# Run with race detector
go test -race ./...

# Race detection for specific package
go test -race ./runtime_test.go
```

### Coverage Analysis

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Coverage for specific package
go test -cover ./internal/agent
```

### Parallel Execution

```bash
# Run tests in parallel (default)
go test ./...

# Control parallelism
go test -parallel 4 ./...
```

### Timeout Configuration

```bash
# Set test timeout (default 10m)
go test -timeout 30s ./...
```

## Writing Tests

### Test Structure

Follow this structure for all tests:

```go
func TestFunctionName(t *testing.T) {
    t.Parallel() // Enable parallel execution when safe

    // Arrange: Set up test data and mocks
    mockRT := agent.NewMockRuntime()
    ctx := agent.ContextWithRuntime(context.Background(), mockRT)

    // Act: Execute the function being tested
    result, err := FunctionUnderTest(ctx, input)

    // Assert: Verify results
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestMultipleScenarios(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   inputType
        want    outputType
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   invalidInput,
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        tt := tt // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            got, err := FunctionUnderTest(tt.input)

            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr %v, got error: %v", tt.wantErr, err)
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("want %v, got %v", tt.want, got)
            }
        })
    }
}
```

### Testing Goroutines

When testing concurrent code:

```go
func TestConcurrentOperation(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    done := make(chan bool)

    go func() {
        // Concurrent operation
        operation()
        done <- true
    }()

    select {
    case <-done:
        // Success
    case <-ctx.Done():
        t.Fatal("operation timed out")
    }
}
```

### Best Practices

1. **Use t.Parallel()**: Enable parallel execution for independent tests
2. **Cleanup Resources**: Use `defer` or `t.Cleanup()` for cleanup
3. **Clear Test Names**: Use descriptive names that explain what's being tested
4. **Test Error Cases**: Always test both success and failure paths
5. **Avoid Sleeps**: Use synchronization primitives instead of time.Sleep
6. **Mock External Dependencies**: Never make real API calls or file I/O in tests
7. **Table-Driven Tests**: Use for testing multiple scenarios
8. **Capture Range Variables**: Use `tt := tt` in table-driven tests with t.Parallel()

## Continuous Integration

Tests should be run on every commit:

```yaml
# Example GitHub Actions workflow
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## Further Reading

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Go Test Best Practices](https://golang.org/doc/effective_go#testing)
