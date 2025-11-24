# Multi-Agent Aggregator Workflow Example

This example demonstrates the power of multi-agent collaboration using the Aggregator agent to synthesize insights from multiple specialized AI agents. The system simulates a research team analyzing a complex topic from different expert perspectives.

## Overview

The example showcases a **Research Synthesis System** where multiple expert agents analyze a topic (e.g., "The Impact of LLMs on Software Development") from their specialized perspectives, and the Aggregator agent combines their insights using various AI-powered strategies.

## Multi-Agent Pattern Demonstrated

### Expert Agent Roles

The system deploys 6 specialized agents, each with unique expertise:

1. **Technical Expert** - Deep technical analysis of implementation details
2. **Data Scientist** - Empirical analysis with metrics and statistics
3. **Business Analyst** - ROI and economic impact assessment
4. **Security Expert** - Security risks and vulnerability analysis
5. **Ethics Expert** - Ethical implications and bias considerations
6. **Domain Expert** - Practical implementation challenges

### Aggregation Process

The workflow demonstrates three distinct aggregation strategies:

#### 1. Consensus Aggregation
- Identifies points of agreement across all experts
- Resolves conflicts using LLM reasoning
- Calculates consensus levels (0-1 scale)
- Highlights areas of strong agreement vs. divergence

#### 2. Semantic Aggregation
- Groups expert insights by semantic similarity
- Creates thematic clusters (e.g., "Technical Aspects", "Business Impact")
- Preserves relationships between related concepts
- Identifies emergent themes across clusters

#### 3. Weighted Aggregation
- Applies expertise-based weights to each agent
- Gives proportional importance based on relevance
- Balances diverse perspectives while emphasizing domain expertise
- Useful when certain experts are more authoritative on specific topics

## How Aggregation Strategies Work

### Consensus Building
The consensus strategy uses advanced NLP to:
- Compare expert opinions for overlapping insights
- Identify contradictions and resolve them with reasoning
- Generate a unified view that represents the collective intelligence
- Calculate statistical consensus metrics

### Semantic Clustering
The semantic strategy leverages text similarity for grouping related insights:
- String similarity algorithms (Levenshtein distance) for comparing text content
- Conceptual grouping of related insights based on text overlap
- Hierarchical clustering for multi-level understanding
- Preservation of nuanced relationships between ideas

**Note**: This implementation uses string-based similarity (Levenshtein distance), not embedding-based semantic similarity. For true semantic understanding using embeddings, consider integrating with a vector database.

### Weighted Synthesis
The weighted approach implements:
- Confidence scoring for each expert input
- Dynamic weight adjustment based on topic relevance
- Balanced integration maintaining minority viewpoints
- Mathematical optimization for optimal synthesis

## Installation & Setup

### Prerequisites

Before running this example, ensure you have the following:

1. **Go Version**: Go 1.21 or higher
   ```bash
   go version  # Should show go1.21 or higher
   ```

2. **API Key**: At least one LLM provider API key
   ```bash
   # For OpenAI (GPT models)
   export OPENAI_API_KEY="sk-your-openai-key-here"

   # OR for Anthropic (Claude models)
   export ANTHROPIC_API_KEY="sk-ant-your-anthropic-key-here"
   ```

3. **Configuration File**: Ensure `config.yaml` exists in the example directory
   ```bash
   ls examples/aggregator-workflow/config.yaml  # Should exist
   ```

### Running the Example

1. Navigate to the example directory:
```bash
cd examples/aggregator-workflow
```

2. Install dependencies (required first time):
```bash
go mod tidy
```

3. Verify environment variables are set:
```bash
echo $OPENAI_API_KEY    # Should display your key
# OR
echo $ANTHROPIC_API_KEY  # Should display your key
```

4. Run with default configuration:
```bash
go run main.go
```

5. Or customize the configuration:
```bash
# Edit config.yaml to modify topic, agents, or strategies
vim config.yaml
go run main.go
```

## Expected Output

The system produces a multi-phase analysis. The examples below show typical output structure and values.

**Note**: All numeric values shown (consensus levels, similarity scores, confidence values, token counts) are illustrative examples and will vary based on the topic being analyzed and the specific responses from the LLM. Actual values depend on the research question, agent perspectives, and model outputs.

### Phase 1: Expert Analysis
```
Starting Multi-Agent Research Synthesis on: The Impact of LLMs on Software Development
Deploying 6 expert agents with different perspectives
Received analysis from Technical Expert agent
Received analysis from Data Scientist agent
...
```

### Phase 2: Strategy-Specific Aggregations
```
=== CONSENSUS AGGREGATION ===
Strategy: consensus
Consensus Level: 0.83
Content Preview: Based on expert consensus, LLMs are fundamentally transforming software development with 83% agreement on key impacts...
Conflicts Resolved: 2 (testing approaches, security protocols)
---

=== SEMANTIC AGGREGATION ===
Strategy: semantic
Semantic Clusters:
  - Cluster 1: Technical Implementation (3 members, 0.85 similarity)
  - Cluster 2: Business & Ethics (2 members, 0.78 similarity)
  - Cluster 3: Security Concerns (1 member, 0.95 confidence)
Content Preview: Semantic analysis reveals three primary themes emerging from expert insights...
---

=== WEIGHTED AGGREGATION ===
Strategy: weighted
Applied Weights: {Technical: 0.9, Security: 0.95, Data: 0.85, ...}
Content Preview: Weighted synthesis emphasizes security and technical considerations as primary concerns...
---
```

### Phase 3: Final Synthesis
```
========================================
FINAL RESEARCH SYNTHESIS
========================================
Topic: The Impact of Large Language Models on Software Development
Consensus Level: 0.83
Total Tokens Used: 8500

Synthesis:
After comprehensive multi-agent analysis, the following key insights emerge:

1. Technical Impact (High Confidence: 0.92)
   - LLMs accelerate development by 40-60% for routine tasks
   - Code quality shows mixed results requiring human oversight
   - Architecture patterns evolving toward AI-first designs

2. Security Considerations (Critical Priority: 0.95)
   - New vulnerability classes from AI-generated code
   - Need for specialized security scanning tools
   - Compliance frameworks lagging behind technology

3. Business Implications (Moderate Confidence: 0.75)
   - ROI positive for organizations above certain scale
   - Skill requirements shifting toward AI collaboration
   - Market disruption in traditional development tools

[... continued comprehensive synthesis ...]

Recommendations:
- Implement staged adoption with security-first approach
- Invest in developer training for AI collaboration
- Establish clear governance frameworks
- Monitor long-term impacts on code maintainability

Areas Requiring Further Research:
- Long-term code maintainability metrics
- Impact on junior developer career paths
- Standardization of AI-assisted development practices
========================================
```

### Saved Output
Results are automatically saved to `research_synthesis_output.json` with full details including:
- All expert analyses
- Aggregation results from each strategy
- Consensus metrics and conflict resolutions
- Semantic clusters and relationships
- Final synthesis and recommendations

#### Output JSON Structure

The saved JSON file follows this structure for easy parsing:

```json
{
  "topic": "The Impact of LLMs on Software Development",
  "timestamp": "2024-01-15T10:30:00Z",
  "expert_analyses": [
    {
      "agent_name": "technical_expert",
      "role": "Technical Expert",
      "analysis": "Detailed technical analysis content...",
      "tokens_used": 850
    }
  ],
  "aggregation_results": {
    "consensus": {
      "strategy": "consensus",
      "consensus_level": 0.83,
      "aggregated_content": "Synthesized consensus view...",
      "conflicts_resolved": [
        {
          "topic": "testing approaches",
          "resolution": "Hybrid approach combining both views..."
        }
      ],
      "tokens_used": 1200
    },
    "semantic": {
      "strategy": "semantic",
      "semantic_clusters": [
        {
          "cluster_id": "cluster_0",
          "members": ["technical_expert", "security_expert"],
          "core_concept": "Technical Implementation",
          "avg_similarity": 0.85
        }
      ],
      "aggregated_content": "Cluster-based synthesis...",
      "tokens_used": 1350
    },
    "weighted": {
      "strategy": "weighted",
      "applied_weights": {
        "technical_expert": 0.9,
        "security_expert": 0.95
      },
      "aggregated_content": "Weighted synthesis...",
      "tokens_used": 1100
    }
  },
  "final_synthesis": {
    "summary": "Comprehensive final synthesis...",
    "key_insights": ["Insight 1", "Insight 2"],
    "recommendations": ["Recommendation 1", "Recommendation 2"],
    "areas_for_research": ["Area 1", "Area 2"]
  },
  "metadata": {
    "total_tokens_used": 8500,
    "processing_time_seconds": 18.5,
    "num_experts": 6
  }
}
```

**Key fields for programmatic access**:
- `aggregation_results.consensus.consensus_level` - Agreement metric (0.0-1.0)
- `expert_analyses[].analysis` - Individual expert perspectives
- `final_synthesis.recommendations` - Actionable recommendations array
- `metadata.total_tokens_used` - Total cost tracking

## When to Use Each Aggregation Strategy

### Use Consensus When:
- You need to find common ground among diverse opinions
- Conflict resolution is important
- Building agreement for decision-making
- Identifying universally accepted insights

### Use Semantic When:
- Understanding thematic relationships is crucial
- You want to preserve conceptual groupings
- Dealing with complex, multi-faceted topics
- Creating comprehensive knowledge maps

### Use Weighted When:
- Some experts have more authority on the topic
- Certain perspectives are more critical
- You need to balance expertise with inclusion
- Making high-stakes decisions requiring domain expertise

### Use Hierarchical When (Future Feature):
- Dealing with very large numbers of agents (>10)
- Multi-level summarization is needed
- Complex organizational structures exist
- Recursive aggregation provides better results

**Status**: Hierarchical aggregation is currently implemented in the core framework but not demonstrated in this example. See `/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/aggregator.go` for implementation details.

### Use RAG-Based When (Future Feature):
- You have a knowledge base to reference
- Historical context is important
- Fact-checking against documentation is needed
- Retrieval of specific information is required

**Status**: RAG-based aggregation is currently implemented in the core framework but not demonstrated in this example. See `/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/aggregator.go` for implementation details.

## Configuration Options

### Modifying the Research Topic
Edit `config.yaml` to analyze different topics:
```yaml
research_topic:
  title: "Your Custom Topic"
  description: "Detailed description..."
  aspects:
    - "Aspect 1"
    - "Aspect 2"
```

### Adjusting Agent Expertise
Customize agent roles and weights:
```yaml
expert_agents:
  - name: "custom_expert"
    role: "Custom Expert"
    expertise: ["Skill 1", "Skill 2"]
    perspective: "Unique viewpoint..."
    weight: 0.8
```

### Fine-Tuning Aggregation
Control aggregation behavior:
```yaml
aggregator:
  consensus_threshold: 0.8  # Require higher agreement
  semantic_similarity: 0.9  # Tighter clustering
  temperature: 0.3          # More focused synthesis
  max_tokens: 3000          # Longer outputs
```

#### Parameter Reference

| Parameter | Type | Range | Default | Description |
|-----------|------|-------|---------|-------------|
| `consensus_threshold` | float | 0.0-1.0 | 0.7 | Minimum agreement level required across agents. Higher values require stronger consensus. |
| `semantic_similarity` | float | 0.0-1.0 | 0.85 | Threshold for grouping similar outputs into clusters. Higher values create tighter, more specific clusters. |
| `temperature` | float | 0.0-2.0 | 0.5 | Controls randomness in LLM synthesis. Lower values (0.2-0.4) are more deterministic, higher values (0.7-1.0) are more creative. |
| `max_tokens` | int | 100-4000 | 1500 | Maximum tokens for aggregated output. Increase for comprehensive synthesis, decrease to reduce costs. |

## Advanced Features

### Semantic Memory Integration
The example includes semantic memory for enhanced analysis:
- Stores key insights for reference
- Enables cross-referencing between analyses
- Improves consistency across aggregations

### Conflict Resolution with LLM Reasoning
When experts disagree:
- Identifies specific points of contention
- Uses LLM to analyze reasoning behind different views
- Generates balanced resolutions with explanations
- Tracks confidence in resolved conflicts

### Dynamic Weight Adjustment
The system can dynamically adjust weights based on:
- Topic relevance
- Historical accuracy
- Confidence scores
- Semantic alignment with query

## Architecture Insights

### AI-Specific Design Decisions

1. **Prompt Engineering**
   - Structured prompts for consistent expert outputs
   - Role-specific system prompts for authentic perspectives
   - Chain-of-thought reasoning for conflict resolution

2. **Token Optimization**
   - Efficient prompt construction to minimize token usage
   - Incremental aggregation for large-scale synthesis
   - Strategic use of temperature for different phases

3. **Semantic Processing**
   - Text similarity calculations for clustering
   - Embedding-based similarity (when available)
   - Hierarchical clustering for complex topics

4. **Consensus Algorithms**
   - Statistical consensus from confidence scores
   - Semantic consensus from content alignment
   - Weighted consensus incorporating expertise

## Extending the Example

### Adding New Aggregation Strategies
Implement custom strategies by modifying the aggregator agent implementation:

1. Add new strategy constants in `/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/aggregator.go`:
   ```go
   const (
       AggregationStrategyConsensus    = "consensus"
       AggregationStrategyWeighted     = "weighted"
       AggregationStrategyYourNewStrategy = "your_new_strategy"  // Add here
   )
   ```

2. Implement the strategy-specific method in the same file:
   ```go
   func (a *AggregatorAgent) processYourNewStrategy(inputs []AgentInput) (string, error) {
       // Your implementation here
   }
   ```

3. Update the configuration schema to include the new strategy
4. Add the strategy to example workflow configurations

### Integrating with Vector Databases
For production systems:
- Store expert analyses as embeddings
- Enable semantic search across historical analyses
- Implement RAG-based aggregation with retrieval
- Build knowledge graphs from aggregated insights

### Multi-Round Refinement
Enhance the system with:
- Iterative refinement based on initial synthesis
- Expert feedback loops for validation
- Progressive consensus building
- Dynamic expert recruitment based on gaps

## Performance Considerations

- **Token Usage**: ~8,000-12,000 tokens for full workflow
- **Latency**: 15-30 seconds for complete analysis
- **Scalability**: Handles 10-20 agents efficiently
- **Memory**: Semantic memory capped at 100 entries

## Troubleshooting

### Common Issues

1. **Invalid API Key Error**
   - Verify your API key is correctly set: `echo $OPENAI_API_KEY` or `echo $ANTHROPIC_API_KEY`
   - Ensure the key is valid and has sufficient credits
   - Check that you're using the correct environment variable for your chosen provider
   - Example fix: `export OPENAI_API_KEY="sk-..."`

2. **Config File Not Found**
   - Ensure `config.yaml` exists in the current directory
   - Check the file path if running from a different location
   - Verify file permissions allow reading
   - Example fix: `ls -la config.yaml` to verify file exists

3. **Go Version Mismatch**
   - This example requires Go 1.21 or higher
   - Check your version: `go version`
   - Upgrade if needed: Visit https://go.dev/doc/install
   - Example output: `go version go1.21.0 darwin/amd64`

4. **Mock Provider Clarification**
   - The mock provider is for testing only and returns simulated responses
   - To use real LLM providers, set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
   - Mock responses are not based on actual AI analysis
   - Switch to real provider for production use

5. **Timeout Errors**
   - Increase `timeout_ms` in configuration
   - Reduce number of expert agents
   - Simplify analysis prompts

6. **Low Consensus Levels**
   - Normal for controversial topics
   - Adjust `consensus_threshold` if needed
   - Review expert perspectives for alignment

7. **API Rate Limits**
   - Implement retry logic with backoff
   - Use mock provider for testing
   - Consider caching for repeated analyses

## Conclusion

This example demonstrates the sophisticated capabilities of multi-agent AI systems with intelligent aggregation. The Aggregator agent serves as a critical component for synthesizing diverse AI perspectives into actionable insights, showcasing the future of collaborative AI systems in research, analysis, and decision-making.