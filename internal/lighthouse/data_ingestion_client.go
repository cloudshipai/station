package lighthouse

import (
	"context"
	"encoding/json"
	"fmt"
	"station/internal/logging"
	"station/pkg/types"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DataIngestionClient handles sending structured data to Lighthouse Data Ingestion service
type DataIngestionClient struct {
	config *LighthouseConfig
	conn   *grpc.ClientConn
}

// DataIngestionServiceClient represents the proto service interface
type DataIngestionServiceClient interface {
	IngestData(ctx context.Context, req *IngestDataRequest, opts ...grpc.CallOption) (*IngestDataResponse, error)
}

// Data ingestion proto message structures (simplified for this implementation)
type IngestDataRequest struct {
	RegistrationKey string            `json:"registration_key"`
	OrganizationId  string            `json:"organization_id"`
	WorkspaceId     string            `json:"workspace_id,omitempty"`
	TableId         string            `json:"table_id,omitempty"`
	App             string            `json:"app"`
	AppType         string            `json:"app_type"`
	SourceId        string            `json:"source_id"`
	Data            *structpb.Struct  `json:"data"`
	Metadata        map[string]string `json:"metadata"`
	Timestamp       *timestamppb.Timestamp `json:"timestamp"`
	CorrelationId   string            `json:"correlation_id"`
}

type IngestDataResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	RecordId        string `json:"record_id"`
	RecordsProcessed int32 `json:"records_processed"`
}

// NewDataIngestionClient creates a new data ingestion client
func NewDataIngestionClient(config *LighthouseConfig) (*DataIngestionClient, error) {
	if config == nil {
		config = DefaultLighthouseConfig()
	}

	return &DataIngestionClient{
		config: config,
	}, nil
}

// Connect establishes connection to the data ingestion service
func (dic *DataIngestionClient) Connect(ctx context.Context) error {
	if dic.conn != nil {
		return nil // Already connected
	}

	conn, err := grpc.DialContext(ctx, dic.config.Endpoint, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to data ingestion service: %w", err)
	}

	dic.conn = conn
	return nil
}

// Close closes the connection
func (dic *DataIngestionClient) Close() error {
	if dic.conn != nil {
		return dic.conn.Close()
	}
	return nil
}

// SendFinOpsData sends agent run data with finops preset to the data ingestion service
func (dic *DataIngestionClient) SendFinOpsData(ctx context.Context, agentRun *types.AgentRun, environment string) error {
	if dic.conn == nil {
		return fmt.Errorf("data ingestion client not connected")
	}

	// Parse the structured response from the agent
	var finopsData map[string]interface{}
	if agentRun.Response != "" {
		if err := json.Unmarshal([]byte(agentRun.Response), &finopsData); err != nil {
			logging.Info("Failed to parse agent response as JSON for finops ingestion: %v", err)
			// Create a simple structure with the response as text
			finopsData = map[string]interface{}{
				"response_text": agentRun.Response,
				"parse_error":   err.Error(),
			}
		}
	}

	// Add metadata about the agent run
	finopsData["agent_run_metadata"] = map[string]interface{}{
		"run_id":      agentRun.ID,
		"agent_id":    agentRun.AgentID,
		"agent_name":  agentRun.AgentName,
		"task":        agentRun.Task,
		"status":      agentRun.Status,
		"duration_ms": agentRun.DurationMs,
		"model_name":  agentRun.ModelName,
		"started_at":  agentRun.StartedAt.Format(time.RFC3339),
		"completed_at": agentRun.CompletedAt.Format(time.RFC3339),
	}

	// Add token usage if available
	if agentRun.TokenUsage != nil {
		finopsData["token_usage"] = map[string]interface{}{
			"prompt_tokens":     agentRun.TokenUsage.PromptTokens,
			"completion_tokens": agentRun.TokenUsage.CompletionTokens,
			"total_tokens":      agentRun.TokenUsage.TotalTokens,
			"cost_usd":         agentRun.TokenUsage.CostUSD,
		}
	}

	// Convert to protobuf Struct
	dataStruct, err := structpb.NewStruct(finopsData)
	if err != nil {
		return fmt.Errorf("failed to convert finops data to protobuf struct: %w", err)
	}

	// Create the ingestion request
	req := &IngestDataRequest{
		RegistrationKey: dic.config.RegistrationKey,
		App:             "finops",
		AppType:         "station-run",
		SourceId:        fmt.Sprintf("station_agent_%s", agentRun.AgentID),
		Data:            dataStruct,
		Metadata: map[string]string{
			"agent_name":     agentRun.AgentName,
			"environment":    environment,
			"preset":         agentRun.OutputSchemaPreset,
			"station_version": "latest", // Could be from version package
			"execution_mode": "agent",
		},
		Timestamp:     timestamppb.New(agentRun.CompletedAt),
		CorrelationId: agentRun.ID,
	}

	logging.Debug("Sending finops data to Lighthouse Data Ingestion service - run_id: %s, agent: %s",
		agentRun.ID, agentRun.AgentName)

	// This would be the actual gRPC call - for now we'll just log
	// In a real implementation, you'd need to generate the proto client
	logging.Info("FinOps data prepared for ingestion: app=%s, app_type=%s, source_id=%s, correlation_id=%s",
		req.App, req.AppType, req.SourceId, req.CorrelationId)

	// TODO: Implement actual gRPC call once proto files are generated
	// client := NewDataIngestionServiceClient(dic.conn)
	// resp, err := client.IngestData(ctx, req)
	// if err != nil {
	//     return fmt.Errorf("failed to send finops data: %w", err)
	// }
	//
	// if !resp.Success {
	//     return fmt.Errorf("finops data ingestion failed: %s", resp.Message)
	// }
	//
	// logging.Debug("Successfully sent finops data - record_id: %s", resp.RecordId)

	return nil
}