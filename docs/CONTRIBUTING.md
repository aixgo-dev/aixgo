# Contributing to aixgo

Thank you for your interest in contributing to aixgo! We welcome contributions
from the community and are excited to work with you.

**For comprehensive documentation, visit [https://aixgo.dev](https://aixgo.dev)**

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Ways to Contribute](#ways-to-contribute)
- [Development Setup](#development-setup)
- [Pull Request Process](#pull-request-process)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Documentation Standards](#documentation-standards)
- [Commit Message Format](#commit-message-format)
- [Copyright and Licensing](#copyright-and-licensing)

## Code of Conduct

This project follows a standard code of conduct. Be respectful, inclusive, and
professional in all interactions.

## Ways to Contribute

### Report Bugs

Found a bug? Please create an issue with:

- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, aixgo version)
- Minimal code sample if applicable

### Suggest Features

Have an idea? Open a discussion or issue with:

- Use case description
- Proposed solution
- Alternative approaches considered
- Why this benefits the broader community

### Submit Code Improvements

We welcome pull requests for:

- Bug fixes
- New agent types
- LLM provider integrations
- Performance optimizations
- Documentation improvements
- Test coverage improvements

### Write Documentation

Documentation contributions are highly valued:

- Tutorial content
- Example applications
- API documentation improvements
- Architecture explanations
- Migration guides

### Share Example Projects

Built something with aixgo? Share it!

- Post in GitHub Discussions "Show & Tell"
- Submit example to the `examples/` directory
- Write a blog post (we'll link to it)

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for convenience commands)

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo

# Install dependencies
go mod download

# Build the project
go build ./...

# Run tests to verify setup
go test ./...
```

### Project Structure

```text
aixgo/
├── agents/           # Agent implementations
├── config/           # Example configurations
├── docs/             # Documentation
├── examples/         # Example applications
├── internal/
│   ├── agent/        # Core agent types and factory
│   ├── llm/          # LLM integration and validation
│   ├── observability/# OpenTelemetry integration
│   └── supervisor/   # Supervisor implementation
├── proto/            # Message protocol
├── aixgo.go          # Main entry point
└── runtime.go        # Communication runtime
```

## Pull Request Process

### 1. Fork and Clone

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/YOUR_USERNAME/aixgo.git
cd aixgo
git remote add upstream https://github.com/aixgo-dev/aixgo.git
```

### 2. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 3. Make Your Changes

- Write clean, idiomatic Go code
- Follow existing code patterns and conventions
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

### 4. Test Your Changes

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Check test coverage
go test -cover ./...

# Run linters
go vet ./...
golangci-lint run
```

### 5. Update Documentation

- Add godoc comments for exported functions and types
- Update README.md if adding features
- Add examples if introducing new patterns
- Update CHANGELOG.md (if we have one)

### 6. Submit Pull Request

- Push your branch to your fork
- Open a PR against `main` branch
- Fill out the PR template completely
- Link any related issues
- Wait for review and address feedback

### PR Checklist

Before submitting, ensure:

- [ ] Tests pass locally (`go test ./...`)
- [ ] No race conditions (`go test -race ./...`)
- [ ] Code is formatted (`gofmt -s -w .`)
- [ ] Linters pass (`go vet ./...`)
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] PR description explains what and why

## Code Style Guidelines

### Go Standards

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt -s` for formatting
- Run `go vet` before committing
- Keep functions focused and small
- Prefer clarity over cleverness

### Naming Conventions

```go
// Package names: lowercase, single word
package agent

// Exported types: PascalCase
type Agent struct {}

// Unexported types: camelCase
type agentImpl struct {}

// Interfaces: noun or adjective
type Runtime interface {}
type Validator interface {}

// Methods: PascalCase for exported, camelCase for unexported
func (a *Agent) Start(ctx context.Context) error {}
func (a *Agent) think(input string) (string, error) {}
```

### Error Handling

```go
// Always check errors explicitly
result, err := operation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use %w for error wrapping
return fmt.Errorf("failed to process: %w", err)

// Provide context in error messages
if err != nil {
    return fmt.Errorf("agent %s failed to start: %w", a.name, err)
}
```

### Comments

```go
// Exported functions must have godoc comments
// explaining what they do, not how they do it.
//
// Example:
//
//	agent := NewAgent(WithName("analyzer"))
//	err := agent.Start(ctx)
func NewAgent(opts ...Option) *Agent {
    // ...
}

// Unexported functions should have comments explaining
// non-obvious logic or design decisions
func (a *Agent) think(input string) (string, error) {
    // Use exponential backoff for retries to avoid
    // overwhelming the LLM API during outages
}
```

## Testing Requirements

### Test Coverage

- Aim for >80% coverage for new code
- 100% coverage for critical paths (validation, factory, runtime)
- Use table-driven tests for multiple scenarios

### Test Structure

```go
func TestAgentStart(t *testing.T) {
    t.Parallel() // Enable parallel execution when safe

    tests := []struct {
        name    string
        input   input
        want    output
        wantErr bool
    }{
        {
            name:    "successful start",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
        // More test cases...
    }

    for _, tt := range tests {
        tt := tt // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            // Test implementation
            got, err := operation(tt.input)

            if (err != nil) != tt.wantErr {
                t.Errorf("wanted error: %v, got: %v", tt.wantErr, err)
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("wanted: %v, got: %v", tt.want, got)
            }
        })
    }
}
```

### Using Mocks

Use the provided test utilities:

```go
// MockRuntime for testing agents
mockRT := agent.NewMockRuntime()
ctx := agent.ContextWithRuntime(context.Background(), mockRT)

// MockOpenAIClient for testing ReAct agents
mockClient := agents.NewMockOpenAIClient()
mockClient.AddResponse(expectedResponse, nil)

// MockFileReader for testing config loading
mockReader := aixgo.NewMockFileReader()
mockReader.AddFile("config.yaml", configData)
```

### Race Detection

Always test with race detector:

```bash
go test -race ./...
```

## Documentation Standards

### Godoc Comments

All exported symbols must have godoc comments:

```go
// Agent represents an autonomous AI agent that processes messages
// and executes tasks within an aixgo system. Agents operate under
// supervisor coordination and communicate via the runtime layer.
//
// Agents support three built-in roles:
//   - Producer: Generates periodic messages
//   - ReAct: Reasoning and acting with LLM integration
//   - Logger: Consumes and logs messages
//
// Example usage:
//
//	agent := NewAgent(
//	    WithName("analyzer"),
//	    WithRole("react"),
//	)
//	if err := agent.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
type Agent struct {
    // ...
}
```

### Markdown Documentation

- Follow [markdownlint](https://github.com/DavidAnson/markdownlint) rules
- Specify language for all code blocks
- Include blank lines after headings
- Keep line length reasonable (<120 chars)
- Use relative links for internal docs

### Code Examples

All examples must:

- Be copy-pasteable and runnable
- Include necessary imports
- Show expected output
- Handle errors explicitly
- Include explanatory comments

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/) format:

```text
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code changes that neither fix bugs nor add features
- `perf`: Performance improvements
- `chore`: Maintenance tasks, dependency updates

### Examples

```text
feat(agents): add classifier agent type

Add a new classifier agent that categorizes incoming messages
based on configurable rules or LLM-based classification.

Closes #42
```

```text
fix(runtime): prevent deadlock in channel send

Fixed race condition in SimpleRuntime.Send() that could cause
deadlock under high concurrency. Added mutex protection around
channel map access.

Fixes #38
```

```text
docs(readme): add installation instructions

Added detailed installation section with examples for different
package managers and build-from-source instructions.
```

## Copyright and Licensing

### License Agreement

- All contributions are licensed under the MIT License
- By submitting a PR, you agree to license your contribution under MIT
- You retain copyright to your contributions
- No Contributor License Agreement (CLA) required

### Headers

No copyright headers required in source files. The repository LICENSE file covers all contributions.

## Getting Help

- **Questions**: Use [GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions)
- **Bugs**: Create an [Issue](https://github.com/aixgo-dev/aixgo/issues)
- **Feature Ideas**: Start a Discussion first, then create an Issue if consensus

## Recognition

Contributors will be recognized:

- In release notes for significant contributions
- In the project README (planned contributors section)
- Via GitHub's built-in contributor tracking

Thank you for contributing to aixgo!
