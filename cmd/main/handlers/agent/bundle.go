package agent

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RunAgentBundleCreate creates a new agent bundle
func (h *AgentHandler) RunAgentBundleCreate(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("agent bundle create not yet implemented")
}

// RunAgentBundleValidate validates an agent bundle
func (h *AgentHandler) RunAgentBundleValidate(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("agent bundle validate not yet implemented")
}

// RunAgentBundleInstall installs an agent bundle
func (h *AgentHandler) RunAgentBundleInstall(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("agent bundle install not yet implemented")
}

// RunAgentBundleDuplicate duplicates an agent across environments
func (h *AgentHandler) RunAgentBundleDuplicate(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("agent bundle duplicate not yet implemented")
}

// RunAgentBundleExport exports an agent as a template bundle
func (h *AgentHandler) RunAgentBundleExport(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("agent bundle export not yet implemented")
}