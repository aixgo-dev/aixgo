package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/pkg/config"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// TicketData represents a customer support ticket
type TicketData struct {
	ID          string    `json:"id"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Customer    string    `json:"customer"`
	Timestamp   time.Time `json:"timestamp"`
}

// ClassificationOutput represents the enhanced classification result
type ClassificationOutput struct {
	TicketID       string                `json:"ticket_id"`
	Classification ClassificationResult  `json:"classification"`
	Routing        RoutingRecommendation `json:"routing"`
	Priority       PriorityAssessment    `json:"priority"`
	SentimentScore float64               `json:"sentiment_score"`
	ProcessedAt    time.Time             `json:"processed_at"`
}

// ClassificationResult from the Classifier agent
type ClassificationResult struct {
	Category       string             `json:"category"`
	Confidence     float64            `json:"confidence"`
	Reasoning      string             `json:"reasoning"`
	Alternatives   []AlternativeClass `json:"alternatives,omitempty"`
	TokensUsed     int                `json:"tokens_used"`
	PromptStrategy string             `json:"prompt_strategy"`
}

// AlternativeClass represents alternative classifications
type AlternativeClass struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

// RoutingRecommendation suggests which team should handle the ticket
type RoutingRecommendation struct {
	Team       string   `json:"team"`
	Expertise  []string `json:"expertise_required"`
	Escalation bool     `json:"escalation_needed"`
}

// PriorityAssessment determines ticket urgency
type PriorityAssessment struct {
	Level   string   `json:"level"` // critical, high, medium, low
	Score   float64  `json:"score"`
	Factors []string `json:"factors"`
}

// SampleTickets provides realistic customer support tickets for demonstration
var SampleTickets = []TicketData{
	{
		ID:          "TICKET-001",
		Subject:     "Cannot access my account after password reset",
		Description: "I reset my password yesterday but now I can't log in. It says my credentials are invalid. I've tried multiple times and even tried resetting again but the email never arrives. This is urgent as I need to access my billing information today.",
		Customer:    "john.doe@example.com",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-002",
		Subject:     "Billing discrepancy on latest invoice",
		Description: "I was charged $299 instead of the $199 that was quoted. My account shows the premium plan but I only signed up for the standard plan. Please review and adjust my billing immediately. Invoice number: INV-2024-0892.",
		Customer:    "jane.smith@company.com",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-003",
		Subject:     "API rate limiting issues in production",
		Description: "We're getting 429 errors from your API even though we're well below our rate limit. Our dashboard shows 5000/10000 requests used but the API is rejecting calls. This is affecting our production environment and causing customer complaints.",
		Customer:    "tech@startup.io",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-004",
		Subject:     "Feature request: Export to Excel functionality",
		Description: "It would be great if we could export our reports directly to Excel format. Currently we have to export as CSV and then convert. Many of our team members would find this useful for their monthly reporting workflows.",
		Customer:    "feedback@enterprise.com",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-005",
		Subject:     "Excellent support experience!",
		Description: "I just wanted to say thank you to Sarah from your support team. She helped me resolve my integration issue quickly and professionally. The documentation she provided was exactly what I needed. Great service!",
		Customer:    "happy@customer.org",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-006",
		Subject:     "Security concern: Suspicious login attempts",
		Description: "I've received multiple notifications about login attempts from IP addresses in different countries. I haven't traveled recently. Please help me secure my account immediately. I'm worried my data might be compromised.",
		Customer:    "security@worried.com",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-007",
		Subject:     "Integration failing with Salesforce",
		Description: "The Salesforce integration stopped syncing data 3 days ago. No error messages in the logs. We've checked our Salesforce permissions and everything looks correct. Need help troubleshooting this ASAP as it's blocking our sales team.",
		Customer:    "ops@salesteam.com",
		Timestamp:   time.Now(),
	},
	{
		ID:          "TICKET-008",
		Subject:     "Slow dashboard performance",
		Description: "The analytics dashboard takes over 30 seconds to load. This started happening after the last update. We have about 100k records. Other pages load fine, just the dashboard is slow. Using Chrome on Windows 11.",
		Customer:    "performance@analytics.net",
		Timestamp:   time.Now(),
	},
}

// WorkflowOrchestrator manages the classification workflow
type WorkflowOrchestrator struct {
	config  *config.Config
	runtime agent.Runtime
	agents  map[string]agent.Agent
	results []ClassificationOutput
}

// NewWorkflowOrchestrator creates a new workflow orchestrator
func NewWorkflowOrchestrator(configPath string) (*WorkflowOrchestrator, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize runtime
	rt := agent.NewDefaultRuntime()

	// Initialize agents
	agents := make(map[string]agent.Agent)
	for _, agentDef := range cfg.Agents {
		if agentDef.Type == "classifier" {
			a, err := agent.CreateAgent(agentDef, rt)
			if err != nil {
				return nil, fmt.Errorf("failed to create agent %s: %w", agentDef.Name, err)
			}
			agents[agentDef.Name] = a
		}
	}

	return &WorkflowOrchestrator{
		config:  cfg,
		runtime: rt,
		agents:  agents,
		results: []ClassificationOutput{},
	}, nil
}

// ProcessTicket sends a ticket through the classification pipeline
func (w *WorkflowOrchestrator) ProcessTicket(ctx context.Context, ticket TicketData) (*ClassificationOutput, error) {
	// Prepare input for classifier
	input := fmt.Sprintf("Subject: %s\n\nDescription: %s\n\nCustomer: %s",
		ticket.Subject, ticket.Description, ticket.Customer)

	// Send to classifier agent via runtime
	inputMsg := &agent.Message{
		Message: &pb.Message{
			Id:        ticket.ID,
			Type:      "ticket",
			Payload:   input,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	// Send through the input channel
	if err := w.runtime.Send("ticket_input", inputMsg); err != nil {
		return nil, fmt.Errorf("failed to send ticket: %w", err)
	}

	// Receive classification result
	resultChan, err := w.runtime.Recv("classification_output")
	if err != nil {
		return nil, fmt.Errorf("failed to setup result channel: %w", err)
	}

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		return w.processClassificationResult(ticket, result)
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("classification timeout for ticket %s", ticket.ID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processClassificationResult enriches the classification with routing and priority
func (w *WorkflowOrchestrator) processClassificationResult(ticket TicketData, msg *agent.Message) (*ClassificationOutput, error) {
	var classification ClassificationResult
	if err := json.Unmarshal([]byte(msg.Payload), &classification); err != nil {
		return nil, fmt.Errorf("failed to parse classification: %w", err)
	}

	// Determine routing based on category
	routing := w.determineRouting(classification)

	// Assess priority based on content and classification
	priority := w.assessPriority(ticket, classification)

	// Calculate sentiment (simplified for demo)
	sentiment := w.analyzeSentiment(ticket.Description)

	output := &ClassificationOutput{
		TicketID:       ticket.ID,
		Classification: classification,
		Routing:        routing,
		Priority:       priority,
		SentimentScore: sentiment,
		ProcessedAt:    time.Now(),
	}

	w.results = append(w.results, *output)
	return output, nil
}

// determineRouting suggests the appropriate team based on classification
func (w *WorkflowOrchestrator) determineRouting(classification ClassificationResult) RoutingRecommendation {
	routingMap := map[string]RoutingRecommendation{
		"technical_issue": {
			Team:       "Technical Support L2",
			Expertise:  []string{"API", "Integration", "Performance"},
			Escalation: classification.Confidence < 0.8,
		},
		"billing_inquiry": {
			Team:       "Billing Department",
			Expertise:  []string{"Invoicing", "Payments", "Subscriptions"},
			Escalation: false,
		},
		"account_access": {
			Team:       "Account Security Team",
			Expertise:  []string{"Authentication", "Password Reset", "Security"},
			Escalation: true, // Always escalate security issues
		},
		"feature_request": {
			Team:       "Product Management",
			Expertise:  []string{"Product Roadmap", "Feature Analysis"},
			Escalation: false,
		},
		"bug_report": {
			Team:       "Engineering Team",
			Expertise:  []string{"Bug Triage", "QA", "Development"},
			Escalation: classification.Confidence > 0.9,
		},
		"positive_feedback": {
			Team:       "Customer Success",
			Expertise:  []string{"Customer Relations", "Feedback Management"},
			Escalation: false,
		},
	}

	if routing, exists := routingMap[classification.Category]; exists {
		return routing
	}

	// Default routing for unknown categories
	return RoutingRecommendation{
		Team:       "General Support",
		Expertise:  []string{"Customer Service"},
		Escalation: classification.Confidence < 0.7,
	}
}

// assessPriority determines ticket priority based on multiple factors
func (w *WorkflowOrchestrator) assessPriority(ticket TicketData, classification ClassificationResult) PriorityAssessment {
	factors := []string{}
	score := 0.5 // Base score

	// Check for urgent keywords
	urgentKeywords := []string{"urgent", "asap", "immediately", "critical", "production", "down", "blocked"}
	for _, keyword := range urgentKeywords {
		if containsIgnoreCase(ticket.Description, keyword) || containsIgnoreCase(ticket.Subject, keyword) {
			score += 0.2
			factors = append(factors, fmt.Sprintf("Contains urgent keyword: %s", keyword))
			break
		}
	}

	// Category-based priority adjustments
	categoryPriority := map[string]float64{
		"account_access":    0.3, // Security issues are high priority
		"technical_issue":   0.2, // Technical issues need quick resolution
		"billing_inquiry":   0.2, // Billing issues affect revenue
		"bug_report":        0.1,
		"feature_request":   -0.1, // Feature requests are lower priority
		"positive_feedback": -0.2, // Feedback is lowest priority
	}

	if adjustment, exists := categoryPriority[classification.Category]; exists {
		score += adjustment
		factors = append(factors, fmt.Sprintf("Category priority adjustment: %s", classification.Category))
	}

	// Confidence-based adjustment
	if classification.Confidence < 0.7 {
		score += 0.1
		factors = append(factors, "Low classification confidence requires review")
	}

	// Normalize score
	if score > 1.0 {
		score = 1.0
	} else if score < 0.0 {
		score = 0.0
	}

	// Determine level based on score
	level := "low"
	if score >= 0.8 {
		level = "critical"
	} else if score >= 0.6 {
		level = "high"
	} else if score >= 0.4 {
		level = "medium"
	}

	return PriorityAssessment{
		Level:   level,
		Score:   score,
		Factors: factors,
	}
}

// analyzeSentiment performs basic sentiment analysis (simplified for demo)
func (w *WorkflowOrchestrator) analyzeSentiment(text string) float64 {
	positiveWords := []string{"thank", "great", "excellent", "happy", "wonderful", "appreciate", "love"}
	negativeWords := []string{"angry", "frustrated", "terrible", "awful", "hate", "disappointed", "unacceptable"}

	score := 0.5 // Neutral baseline
	wordCount := 0

	for _, word := range positiveWords {
		if containsIgnoreCase(text, word) {
			score += 0.1
			wordCount++
		}
	}

	for _, word := range negativeWords {
		if containsIgnoreCase(text, word) {
			score -= 0.1
			wordCount++
		}
	}

	// Normalize score
	if score > 1.0 {
		score = 1.0
	} else if score < 0.0 {
		score = 0.0
	}

	return score
}

// containsIgnoreCase checks if text contains substring (case-insensitive)
func containsIgnoreCase(text, substr string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}

// GenerateReport creates a summary report of all classifications
func (w *WorkflowOrchestrator) GenerateReport() string {
	report := "Customer Support Ticket Classification Report\n"
	report += "=" + strings.Repeat("=", 50) + "\n\n"
	report += fmt.Sprintf("Total Tickets Processed: %d\n", len(w.results))
	report += fmt.Sprintf("Processing Time: %s\n\n", time.Now().Format(time.RFC3339))

	// Category distribution
	categoryCount := make(map[string]int)
	totalConfidence := 0.0
	priorityCount := make(map[string]int)

	for _, result := range w.results {
		categoryCount[result.Classification.Category]++
		totalConfidence += result.Classification.Confidence
		priorityCount[result.Priority.Level]++
	}

	report += "Category Distribution:\n"
	for category, count := range categoryCount {
		percentage := float64(count) * 100 / float64(len(w.results))
		report += fmt.Sprintf("  - %s: %d tickets (%.1f%%)\n", category, count, percentage)
	}

	report += "\nPriority Distribution:\n"
	for priority, count := range priorityCount {
		percentage := float64(count) * 100 / float64(len(w.results))
		report += fmt.Sprintf("  - %s: %d tickets (%.1f%%)\n", priority, count, percentage)
	}

	if len(w.results) > 0 {
		avgConfidence := totalConfidence / float64(len(w.results))
		report += fmt.Sprintf("\nAverage Classification Confidence: %.2f\n", avgConfidence)
	}

	report += "\nDetailed Results:\n"
	report += "-" + strings.Repeat("-", 50) + "\n"

	for _, result := range w.results {
		report += fmt.Sprintf("\nTicket ID: %s\n", result.TicketID)
		report += fmt.Sprintf("  Category: %s (Confidence: %.2f)\n",
			result.Classification.Category, result.Classification.Confidence)
		report += fmt.Sprintf("  Priority: %s (Score: %.2f)\n",
			result.Priority.Level, result.Priority.Score)
		report += fmt.Sprintf("  Routing: %s\n", result.Routing.Team)
		report += fmt.Sprintf("  Sentiment Score: %.2f\n", result.SentimentScore)

		if result.Classification.Reasoning != "" {
			report += fmt.Sprintf("  AI Reasoning: %s\n", result.Classification.Reasoning)
		}

		if len(result.Classification.Alternatives) > 0 {
			report += "  Alternative Classifications:\n"
			for _, alt := range result.Classification.Alternatives {
				report += fmt.Sprintf("    - %s (%.2f)\n", alt.Category, alt.Confidence)
			}
		}
	}

	return report
}

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("Customer Support Ticket Classifier - AI-Powered Workflow")
	fmt.Println("=========================================================\n")

	// Load configuration
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Initialize workflow
	workflow, err := NewWorkflowOrchestrator(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize workflow: %v", err)
	}

	// Start classifier agents
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for name, agent := range workflow.agents {
		go func(n string, a agent.Agent) {
			if err := a.Start(ctx); err != nil {
				log.Printf("Agent %s error: %v", n, err)
			}
		}(name, agent)
	}

	// Allow agents to initialize
	time.Sleep(2 * time.Second)

	// Process sample tickets
	fmt.Println("Processing sample customer support tickets...\n")

	for i, ticket := range SampleTickets {
		fmt.Printf("[%d/%d] Processing ticket %s: %s\n",
			i+1, len(SampleTickets), ticket.ID, ticket.Subject)

		result, err := workflow.ProcessTicket(ctx, ticket)
		if err != nil {
			log.Printf("Error processing ticket %s: %v", ticket.ID, err)
			continue
		}

		// Display immediate result
		fmt.Printf("  ✓ Classified as: %s (Confidence: %.2f)\n",
			result.Classification.Category, result.Classification.Confidence)
		fmt.Printf("  → Routed to: %s (Priority: %s)\n",
			result.Routing.Team, result.Priority.Level)

		if len(result.Priority.Factors) > 0 {
			fmt.Printf("  → Priority factors: %s\n", strings.Join(result.Priority.Factors, ", "))
		}

		fmt.Println()
	}

	// Generate and display report
	fmt.Println("\nGenerating final report...")
	report := workflow.GenerateReport()
	fmt.Println(report)

	// Save results to file
	resultsFile := "classification_results.json"
	resultsJSON, err := json.MarshalIndent(workflow.results, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal results: %v", err)
	} else {
		if err := os.WriteFile(resultsFile, resultsJSON, 0644); err != nil {
			log.Printf("Failed to save results: %v", err)
		} else {
			fmt.Printf("\nResults saved to %s\n", resultsFile)
		}
	}

	fmt.Println("\nWorkflow completed successfully!")
}
