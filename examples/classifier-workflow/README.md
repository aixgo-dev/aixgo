# Customer Support Ticket Classification Workflow

This example demonstrates a production-ready AI-powered customer support ticket classification system using the aixgo Classifier agent. The system leverages advanced LLM capabilities including structured outputs, chain-of-thought reasoning, confidence scoring, and few-shot learning to accurately categorize and route support tickets.

## Overview

The workflow processes customer support tickets through an intelligent classification pipeline that:

1. **Classifies tickets** into predefined categories using semantic understanding
2. **Provides confidence scores** for each classification decision
3. **Generates AI reasoning** explaining the classification logic
4. **Routes tickets** to appropriate teams based on category
5. **Assigns priority levels** based on content analysis and urgency indicators
6. **Performs sentiment analysis** to gauge customer satisfaction
7. **Suggests alternatives** when classification confidence is low

## Features

### AI/LLM Capabilities

- **Structured Output Generation**: The classifier uses JSON schema validation to ensure consistent, parseable responses
- **Chain-of-Thought Reasoning**: The AI provides detailed explanations for its classification decisions
- **Few-Shot Learning**: Pre-configured examples improve classification accuracy for common patterns
- **Confidence Scoring**: Each classification includes a confidence score (0-1) for quality assessment
- **Multi-Label Support**: Can be configured to assign multiple categories to complex tickets
- **Alternative Classifications**: Provides secondary category suggestions when primary confidence is low
- **Semantic Understanding**: Goes beyond keyword matching to understand context and intent

### Classification Categories

1. **Technical Issue**: API errors, performance problems, system failures
2. **Billing Inquiry**: Payment questions, invoice discrepancies, subscription changes
3. **Account Access**: Login problems, password resets, security concerns
4. **Feature Request**: Product suggestions, enhancement ideas
5. **Bug Report**: Detailed reports of system defects
6. **Positive Feedback**: Customer testimonials, satisfaction expressions
7. **General Inquiry**: Other questions about products or services

## Installation and Setup

### Prerequisites

- Go 1.21 or higher
- Access to an LLM provider (OpenAI, Anthropic, etc.)
- API keys configured in environment variables

### Environment Variables

```bash
# For OpenAI
export OPENAI_API_KEY="your-api-key"

# For Anthropic Claude
export ANTHROPIC_API_KEY="your-api-key"

# For other providers, see documentation
```

### Running the Example

1. Navigate to the example directory:
```bash
cd examples/classifier-workflow
```

2. Run with default configuration:
```bash
go run main.go
```

3. Or specify a custom configuration:
```bash
go run main.go custom-config.yaml
```

## Configuration

The `config.yaml` file contains comprehensive settings for the classification workflow:

### Key Configuration Options

#### Classifier Configuration
```yaml
classifier_config:
  confidence_threshold: 0.7  # Minimum confidence for automatic routing
  temperature: 0.3           # LLM temperature (0-1, lower = more consistent)
  multi_label: false         # Enable/disable multi-category assignment
  max_tokens: 500           # Maximum tokens for reasoning response
```

#### Few-Shot Examples
Provide examples to improve classification accuracy:
```yaml
few_shot_examples:
  - input: "Cannot log into my account"
    category: "account_access"
    reason: "Authentication and login issues indicate account access problems"
```

#### Category Definitions
Each category includes:
- **name**: Category identifier
- **description**: Semantic description for LLM understanding
- **keywords**: Terms that suggest this category
- **examples**: Sample tickets for this category

### Advanced Configuration

#### Routing Rules
Define team assignments and SLAs:
```yaml
routing:
  rules:
    - category: technical_issue
      team: "Technical Support L2"
      sla_hours: 4
      auto_escalate: true
```

#### Observability Settings
Monitor AI performance:
```yaml
observability:
  track_token_usage: true
  log_confidence_scores: true
  alert_threshold: 0.5
```

## Expected Output

The workflow produces detailed classification results for each ticket:

```
Processing ticket TICKET-001: Cannot access my account after password reset
  ✓ Classified as: account_access (Confidence: 0.92)
  → Routed to: Security Team (Priority: high)
  → Priority factors: Contains urgent keyword: urgent, Category priority adjustment

AI Reasoning: The customer is experiencing authentication issues after a password reset,
with emails not arriving. This is a clear account access problem requiring immediate
security team attention.

Alternative Classifications:
  - technical_issue (0.45)
  - general_inquiry (0.23)
```

### Output Files

1. **classification_results.json**: Detailed JSON output with all classification data
2. **Console Report**: Summary statistics and performance metrics

### Report Metrics

The final report includes:
- Category distribution percentages
- Priority level breakdown
- Average classification confidence
- Token usage statistics
- Processing latency metrics
- Detailed reasoning for each classification

## Adapting for Your Use Case

### 1. Custom Categories

Modify the `categories` section in `config.yaml`:

```yaml
categories:
  - name: your_category
    description: "Detailed description for LLM understanding"
    keywords: ["relevant", "terms"]
    examples: ["Example ticket text"]
```

### 2. Different LLM Models

Change the model in the agent configuration:

```yaml
agents:
  - name: ticket_classifier
    model: claude-3-opus  # or gpt-4-turbo, etc.
```

### 3. Custom Routing Logic

Implement your routing rules in the `determineRouting` function in `main.go`:

```go
func (w *WorkflowOrchestrator) determineRouting(classification ClassificationResult) RoutingRecommendation {
    // Add your custom routing logic here
}
```

### 4. Integration with Existing Systems

The workflow can be integrated with:
- **Ticketing Systems**: Zendesk, Freshdesk, ServiceNow
- **Communication Platforms**: Slack, Microsoft Teams
- **CRM Systems**: Salesforce, HubSpot
- **Monitoring Tools**: Datadog, New Relic

Example integration pattern:
```go
// Fetch tickets from your system
tickets := fetchFromTicketingSystem()

// Process through classifier
for _, ticket := range tickets {
    result, _ := workflow.ProcessTicket(ctx, ticket)

    // Update your system with classification
    updateTicketingSystem(ticket.ID, result)
}
```

### 5. Multi-Label Classification

Enable multi-label support for complex tickets:

```yaml
classifier_config:
  multi_label: true  # Tickets can have multiple categories
```

### 6. Confidence Threshold Tuning

Adjust thresholds based on your accuracy requirements:

```yaml
classifier_config:
  confidence_threshold: 0.85  # Higher threshold for critical systems
```

## Performance Optimization

### Token Usage Optimization

1. **Limit few-shot examples**: Use 3-5 most relevant examples
2. **Concise descriptions**: Keep category descriptions focused
3. **Selective reasoning**: Request reasoning only for low-confidence results

### Latency Reduction

1. **Caching**: Implement prompt caching for repeated classifications
2. **Batch processing**: Process multiple tickets in parallel
3. **Model selection**: Use faster models for initial triage

### Cost Management

Monitor and optimize token usage:
```yaml
observability:
  track_token_usage: true
  cost_alert_threshold: 100  # Alert when daily cost exceeds $100
```

## Troubleshooting

### Common Issues

1. **Low Classification Confidence**
   - Add more few-shot examples
   - Refine category descriptions
   - Check for overlapping categories

2. **Incorrect Classifications**
   - Review and update keywords
   - Adjust temperature settings
   - Provide clearer category boundaries

3. **High Latency**
   - Reduce max_tokens
   - Use a faster model
   - Implement caching

4. **Token Limit Exceeded**
   - Reduce few-shot examples
   - Shorten category descriptions
   - Lower max_tokens setting

## Best Practices

1. **Regular Evaluation**: Monitor classification accuracy and adjust configurations
2. **Feedback Loop**: Use misclassified tickets to improve few-shot examples
3. **A/B Testing**: Test different prompts and models for optimal performance
4. **Security**: Sanitize ticket content before processing
5. **Compliance**: Ensure PII handling meets regulatory requirements

## Advanced Features

### Custom Prompt Engineering

Modify the system prompt for specific domains:
```yaml
agents:
  - prompt: |
      You are specialized in healthcare support tickets.
      Consider HIPAA compliance and patient privacy.
      Prioritize life-critical issues.
```

### Embedding-Based Classification

Enable semantic similarity matching:
```yaml
classifier_config:
  use_embeddings: true  # Requires embedding provider configuration
```

### Dynamic Category Learning

The system can be extended to learn new categories from patterns:
```go
// Pseudo-code for dynamic learning
func (w *WorkflowOrchestrator) learnNewCategory(patterns []TicketData) {
    // Analyze patterns
    // Generate category definition
    // Update configuration
}
```

## Contributing

To contribute improvements to this example:

1. Fork the repository
2. Create a feature branch
3. Implement your changes
4. Add tests for new functionality
5. Submit a pull request

## Support

For questions or issues:
- Check the aixgo documentation
- Review the Classifier agent implementation
- Open an issue on GitHub
- Contact the support team

## License

This example is part of the aixgo project and follows the same license terms.