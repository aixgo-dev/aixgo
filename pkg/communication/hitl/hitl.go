// Package hitl provides a Human-In-The-Loop interface abstraction.
// This package enables AI agents to request human approval for decisions,
// send notifications, and receive commands from humans via various platforms.
//
// Implementations can be built for different platforms:
//   - telegram: Telegram Bot API (for universal access)
//   - googlechat: Google Chat API (for Google Workspace users)
//   - slack: Slack API (for Slack workspaces)
//   - cli: Command-line interface (for local development)
package hitl

import (
	"context"
	"time"
)

// Interface defines the contract for human-in-the-loop communication.
// Both Telegram and Google Chat implementations satisfy this interface.
type Interface interface {
	// Lifecycle
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Messaging
	SendMessage(ctx context.Context, text string) error
	SendMarkdown(ctx context.Context, markdown string) error

	// Approvals
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResult, error)

	// Notifications (broadcast to all allowed users/spaces)
	Notify(ctx context.Context, message string) error
	NotifyMarkdown(ctx context.Context, markdown string) error

	// Commands
	RegisterCommand(command string, handler CommandHandler)

	// Status
	SetStatusProvider(provider StatusProvider)
}

// CommandHandler handles a user command.
type CommandHandler func(ctx context.Context, cmd *Command) error

// Command represents an incoming command from a user.
type Command struct {
	Name      string   // Command name without leading /
	Args      []string // Arguments after the command
	UserID    string   // Platform-specific user identifier
	UserEmail string   // Email if available (Google Chat)
	UserName  string   // Display name
	ChatID    string   // Platform-specific chat/space identifier
	Timestamp time.Time
}

// ApprovalRequest represents a decision requiring human approval.
type ApprovalRequest struct {
	ID          string           // Unique identifier for this request
	Title       string           // Short title
	Description string           // Detailed description
	Details     string           // Optional additional details (shown separately)
	Options     []ApprovalOption // Available choices
	Timeout     time.Duration    // How long to wait for response (0 = no timeout)
}

// ApprovalOption is a single approval choice.
type ApprovalOption struct {
	Label string // Display text (e.g., "Approve")
	Value string // Machine-readable value (e.g., "approve")
	Style string // Optional: "primary", "danger", "secondary"
}

// ApprovalResult represents the human's decision.
type ApprovalResult struct {
	RequestID string
	Decision  string // The Value from the chosen ApprovalOption
	Comment   string // Optional comment from the user
	UserID    string // Who made the decision
	UserEmail string // Email if available
	UserName  string // Display name
	Timestamp time.Time
}

// StatusProvider returns the current agent status for /status command.
type StatusProvider interface {
	GetStatus(ctx context.Context) (*AgentStatus, error)
}

// AgentStatus represents the current state of the agent.
type AgentStatus struct {
	State            string    // "running", "paused", "error"
	CurrentTask      string    // What the agent is currently doing
	LastActivity     time.Time // When the agent last performed an action
	PendingApprovals int       // Number of approvals waiting
	TasksCompleted   int       // Total tasks completed
	TasksFailed      int       // Total tasks failed
	Uptime           time.Duration
	Version          string
}

// Common approval options for convenience.
var (
	ApproveRejectOptions = []ApprovalOption{
		{Label: "Approve", Value: "approve", Style: "primary"},
		{Label: "Reject", Value: "reject", Style: "danger"},
	}

	ApproveRejectDeferOptions = []ApprovalOption{
		{Label: "Approve", Value: "approve", Style: "primary"},
		{Label: "Reject", Value: "reject", Style: "danger"},
		{Label: "Defer", Value: "defer", Style: "secondary"},
	}

	YesNoOptions = []ApprovalOption{
		{Label: "Yes", Value: "yes", Style: "primary"},
		{Label: "No", Value: "no", Style: "secondary"},
	}
)

// NewApprovalRequest creates a new approval request with sensible defaults.
func NewApprovalRequest(id, title, description string) ApprovalRequest {
	return ApprovalRequest{
		ID:          id,
		Title:       title,
		Description: description,
		Options:     ApproveRejectOptions,
		Timeout:     0, // No timeout by default
	}
}

// WithOptions sets the options for the approval request.
func (r ApprovalRequest) WithOptions(options []ApprovalOption) ApprovalRequest {
	r.Options = options
	return r
}

// WithTimeout sets the timeout for the approval request.
func (r ApprovalRequest) WithTimeout(timeout time.Duration) ApprovalRequest {
	r.Timeout = timeout
	return r
}

// WithDetails adds additional details to the approval request.
func (r ApprovalRequest) WithDetails(details string) ApprovalRequest {
	r.Details = details
	return r
}

// IsApproved returns true if the decision was to approve.
func (r ApprovalResult) IsApproved() bool {
	return r.Decision == "approve" || r.Decision == "yes"
}

// IsRejected returns true if the decision was to reject.
func (r ApprovalResult) IsRejected() bool {
	return r.Decision == "reject" || r.Decision == "no"
}

// IsDeferred returns true if the decision was to defer.
func (r ApprovalResult) IsDeferred() bool {
	return r.Decision == "defer"
}
