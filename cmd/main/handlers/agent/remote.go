package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"station/pkg/models"
)

// Remote agent operations

func (h *AgentHandler) listAgentsRemote(endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents", endpoint)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Agents []*models.Agent `json:"agents"`
		Count  int             `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("• No agents found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d agent(s):\n", result.Count)
	for _, agent := range result.Agents {
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(agent.Name), agent.ID)
		if agent.Description != "" {
			fmt.Printf(" - %s", agent.Description)
		}
		fmt.Printf(" [Environment: %d, Max Steps: %d]\n", agent.EnvironmentID, agent.MaxSteps)
	}

	return nil
}

func (h *AgentHandler) showAgentRemote(agentID int64, endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents/%d", endpoint, agentID)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Agent *models.Agent `json:"agent"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	agent := result.Agent
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Agent: %s\n", styles.Success.Render(agent.Name))
	fmt.Printf("ID: %d\n", agent.ID)
	fmt.Printf("Description: %s\n", agent.Description)
	fmt.Printf("Environment ID: %d\n", agent.EnvironmentID)
	fmt.Printf("Max Steps: %d\n", agent.MaxSteps)
	if agent.CronSchedule != nil {
		fmt.Printf("Schedule: %s (Enabled: %t)\n", *agent.CronSchedule, agent.ScheduleEnabled)
	}
	fmt.Printf("Created: %s\n", agent.CreatedAt.Format("Jan 2, 2006 15:04"))
	fmt.Printf("Updated: %s\n", agent.UpdatedAt.Format("Jan 2, 2006 15:04"))

	// Get recent runs for this agent
	runsURL := fmt.Sprintf("%s/api/v1/runs/agent/%d", endpoint, agentID)
	runsReq, err := makeAuthenticatedRequest(http.MethodGet, runsURL, nil)
	if err == nil {
		client := &http.Client{}
		runsResp, err := client.Do(runsReq)
		if err == nil && runsResp.StatusCode == http.StatusOK {
			defer runsResp.Body.Close()
			var runsResult struct {
				Runs  []*models.AgentRun `json:"runs"`
				Count int                `json:"count"`
			}
			if json.NewDecoder(runsResp.Body).Decode(&runsResult) == nil && len(runsResult.Runs) > 0 {
				fmt.Printf("\nRecent runs (%d):\n", runsResult.Count)
				for i, run := range runsResult.Runs {
					if i >= 5 { // Show only last 5 runs
						break
					}
					fmt.Printf("• Run %d: %s [%s]\n", run.ID, run.Status, run.StartedAt.Format("Jan 2 15:04"))
				}
			}
		}
	}

	return nil
}

func (h *AgentHandler) runAgentRemote(agentID int64, task string, endpoint string, tail bool) error {
	runRequest := struct {
		Task string `json:"task"`
	}{
		Task: task,
	}

	jsonData, err := json.Marshal(runRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/agents/%d/execute", endpoint, agentID)
	req, err := makeAuthenticatedRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("🚀 Executing agent %d with task: %s\n", agentID, styles.Info.Render(task))

	if tail {
		// For remote tail, we'll need to implement polling or WebSocket
		// For now, we'll do a simple execution and show result
		fmt.Println(styles.Error.Render("⚠️  Tail mode not yet implemented for remote agents"))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AgentID  int64  `json:"agent_id"`
		Task     string `json:"task"`
		Response string `json:"response"`
		Success  bool   `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("✅ Agent execution completed\n")
	fmt.Printf("Response: %s\n", result.Response)

	return nil
}

func (h *AgentHandler) deleteAgentRemote(agentID int64, endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents/%d", endpoint, agentID)
	req, err := makeAuthenticatedRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Agent deleted: %s\n", styles.Success.Render(fmt.Sprintf("ID %d", agentID)))

	return nil
}