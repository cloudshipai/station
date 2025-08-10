package file_config

import (
	"github.com/spf13/cobra"
)

// environmentsCommand manages MCP environments
func (h *FileConfigHandler) environmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environments",
		Short: "Manage MCP environments",
		Long:  "Create, list, update, and delete environments for MCP configurations.",
		Aliases: []string{"env"},
	}

	// Add environment subcommands
	cmd.AddCommand(h.listEnvironmentsCommand())
	cmd.AddCommand(h.createEnvironmentCommand())
	cmd.AddCommand(h.updateEnvironmentCommand())
	cmd.AddCommand(h.deleteEnvironmentCommand())
	cmd.AddCommand(h.getEnvironmentCommand())
	
	return cmd
}