# Quick Start Implementation Guide

**TL;DR**: Start here for a fast overview of what to implement first.

---

## Critical Blockers (Fix First!)

### 1. API Key Management (1-2 hours) ⚠️ URGENT

**Problem**: Hardcoded placeholder API keys everywhere

**Files**:
- `/Users/charlesgreen/github.com/aixgo-dev/aixgo/agents/react.go:55`
- `/Users/charlesgreen/github.com/aixgo-dev/aixgo/internal/supervisor/supervisor.go:39-42`

**Current Code**:
```go
client = openai.NewClient("xai-api-key-placeholder")
```

**Fix**:
```go
func getAPIKey() string {
    key := os.Getenv("XAI_API_KEY")
    if key == "" {
        key = os.Getenv("OPENAI_API_KEY")
    }
    if key == "" {
        log.Fatal("No API key found. Set XAI_API_KEY or OPENAI_API_KEY")
    }
    return key
}
```

**Impact**: Enables all real LLM usage

---

### 2. HuggingFace Provider Setup (2-4 hours) ⚠️ CRITICAL

**Problem**: HuggingFace models configured but provider never created

**File**: `/Users/charlesgreen/github.com/aixgo-dev/aixgo/aixgo.go:299-321`

**Current Code**:
```go
func setupHuggingFaceProvider(...) error {
    log.Printf("HuggingFace provider setup would be done here")
    setter.SetProvider(nil) // Placeholder
    return nil
}
```

**Fix**: See detailed implementation in IMPLEMENTATION_ROADMAP.md Section 1.2.1

**Key Steps**:
1. Look up model service from config by model name
2. Create inference service based on runtime (Ollama/vLLM/cloud)
3. Create `OptimizedHuggingFaceProvider` with inference service
4. Call `setter.SetProvider(prov)`

**Impact**: Enables HuggingFace model usage (core feature)

---

### 3. Input Validation (4-6 hours) ⚠️ SECURITY

**Problem**: Incomplete validation allows injection attacks

**Files**: Various tool handlers, MCP server, ReAct parser

**What to Add**:
- Path traversal checks (`../` blocking)
- Command injection prevention (no shell execution with user input)
- String length limits
- Type validation
- Allowlist validation

**Impact**: Prevents security vulnerabilities

---

## High-Priority Features (Do Next)

### 4. Type-Safe Tool Registration (3-5 hours)

**Problem**: Manual schema definition required for every tool

**File**: `/Users/charlesgreen/github.com/aixgo-dev/aixgo/pkg/mcp/server.go:112-132`

**What to Implement**:
- Auto-generate JSON schema from Go struct types using reflection
- Auto-unmarshal `Args` to typed input structs
- Validation based on struct tags

**Impact**: Better developer experience, fewer errors

---

### 5. Cloud Inference Service (4-6 hours)

**Problem**: Only Ollama works, no cloud HuggingFace API support

**What to Build**:
- HuggingFace Inference API HTTP client
- Token authentication
- Rate limiting and retries
- Implement `InferenceService` interface

**Impact**: Enables cloud deployment without local models

---

### 6. Production Authentication (8-12 hours)

**Problem**: No authentication system (security risk)

**File**: `/Users/charlesgreen/github.com/aixgo-dev/aixgo/pkg/security/auth.go`

**What to Implement**:
- Bearer token validation
- API key management system
- Middleware for authentication
- Token expiration
- Multi-tenancy support

**Impact**: Production security requirement

---

## Medium-Priority Features (Later)

### 7. Authorization & RBAC (6-10 hours)
- Role-based access control
- Tool-level permissions
- Admin separation

### 8. Streaming Support (4-6 hours)
- Server-Sent Events parsing
- Chunk-by-chunk processing
- Real-time output

### 9. Structured Output (6-8 hours)
- JSON schema validation for HF models
- Constrained decoding
- Retry logic

### 10. gRPC Transport (8-16 hours)
- Protocol buffer definitions
- gRPC server/client
- TLS support

---

## Implementation Order (Recommended)

### Week 1: Foundation
1. API Key Management (Critical) - 1-2h
2. HuggingFace Provider Setup (Critical) - 2-4h
3. Input Validation (Security) - 4-6h

**Outcome**: Working HuggingFace + Ollama with basic security

---

### Week 2: Security
4. LLM Security (Prompt Injection) - 3-5h
5. Authentication System - 8-12h
6. Authorization & RBAC - 6-10h

**Outcome**: Production-ready security

---

### Week 3: Developer Experience
7. Type-Safe Tool Registration - 3-5h
8. Audit Logging Integration - 2-3h
9. Cloud Inference Service - 4-6h

**Outcome**: Enhanced usability and cloud support

---

### Week 4: Advanced Features
10. Streaming Support - 4-6h
11. Structured Output - 6-8h
12. vLLM Runtime - 4-6h

**Outcome**: Full-featured production system

---

## Quick Reference: All Stubs by Priority

### CRITICAL (Blocks Production)
- [ ] API Key Management - `agents/react.go:55`, `supervisor.go:39`
- [ ] HuggingFace Provider Setup - `aixgo.go:299-321`
- [ ] Input Validation - Various files
- [ ] LLM Security (Prompt Injection) - ReAct parser
- [ ] Authentication System - `pkg/security/auth.go`
- [ ] Authorization & RBAC - `pkg/security/auth.go`

### HIGH (Core Functionality)
- [ ] Type-Safe Tool Registration - `pkg/mcp/server.go:112-132`
- [ ] Cloud Inference Service - Model service config
- [ ] Audit Logging Integration - Security framework

### MEDIUM (Enhanced Features)
- [ ] gRPC Transport - `pkg/mcp/transport_grpc.go`
- [ ] vLLM Runtime Client - Model service runtime
- [ ] Structured Output - `huggingface_production.go:589`
- [ ] Streaming Support - `huggingface_production.go:593`

### LOW (Future)
- [ ] Agent Config Unmarshaling - `internal/agent/types.go:51`
- [ ] K8s Image Registry Config - `deploy/k8s/README.md`
- [ ] Benchmark Evaluation Suite - `internal/llm/evaluation/`

---

## Testing Checklist

For each feature implementation:

- [ ] Unit tests written (80%+ coverage)
- [ ] Integration tests added
- [ ] Security tests (if applicable)
- [ ] Performance tests (if applicable)
- [ ] Documentation updated
- [ ] Examples added
- [ ] Code reviewed

---

## Success Criteria

### Phase 1 (Week 1) ✅
- [ ] Can run HuggingFace models with Ollama
- [ ] No hardcoded API keys
- [ ] Basic input validation in place
- [ ] Integration test passes

### Phase 2 (Week 2) ✅
- [ ] Authentication working
- [ ] RBAC controls access
- [ ] Audit logs functional
- [ ] Security tests pass

### Phase 3 (Week 3) ✅
- [ ] Type-safe tools in use
- [ ] Cloud inference working
- [ ] Documentation complete

### Phase 4 (Week 4) ✅
- [ ] Streaming responses
- [ ] Structured output
- [ ] Production deployment ready

---

## Get Help

- Full details: See `IMPLEMENTATION_ROADMAP.md`
- Security checklist: See `PRODUCTION_SECURITY_CHECKLIST.md`
- Contributing: See `docs/CONTRIBUTING.md`

---

## Start Now!

```bash
# 1. Set up environment
export OPENAI_API_KEY="your-key-here"
export XAI_API_KEY="your-xai-key-here"  # or use OpenAI key

# 2. Fix API key management (replace hardcoded keys)
# Edit: agents/react.go, internal/supervisor/supervisor.go

# 3. Test with Ollama
docker run -d -p 11434:11434 ollama/ollama
ollama pull gemma:2b

# 4. Wire up HuggingFace provider
# Edit: aixgo.go:setupHuggingFaceProvider

# 5. Run integration test
go test -v ./...

# 6. Celebrate! 🎉
```

---

**Last Updated**: 2025-11-21
**Next Review**: After Phase 1 completion
