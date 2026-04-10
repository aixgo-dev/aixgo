package cmd

import (
	"context"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/assistant/session"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/spf13/cobra"
)

// completeModelNames provides dynamic shell completion for the --model flag.
// It lists model IDs from configured providers with a short timeout so
// completion stays responsive even when provider APIs are slow.
func completeModelNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(provider.GetAvailableProviderNames()) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	models, err := provider.ListAllModels(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	models = provider.FilterChatModels(models)
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

// completeSessionIDs provides dynamic shell completion for session IDs.
func completeSessionIDs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	mgr, err := session.NewManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	sessions, err := mgr.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}
