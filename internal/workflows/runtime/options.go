package runtime

import (
	"os"
	"strconv"
)

// Options controls how the workflow runtime connects to NATS/JetStream.
type Options struct {
	Enabled        bool
	URL            string
	Stream         string
	SubjectPrefix  string
	ConsumerName   string
	Embedded       bool
	EmbeddedPort   int // Port for embedded NATS server (default: 4222)
	WorkerPoolSize int // Number of concurrent workflow step processors (default: 10)
}

const defaultNATSURL = "nats://127.0.0.1:4222"

// EnvOptions builds runtime options from environment variables.
// WORKFLOW_NATS_ENABLED=true enables the engine.
// WORKFLOW_NATS_URL overrides the NATS connection string. If set to a non-default value,
// embedded NATS is automatically disabled and Station connects to the external server.
// WORKFLOW_NATS_STREAM sets the JetStream stream name (default: WORKFLOW_EVENTS).
// WORKFLOW_NATS_SUBJECT_PREFIX sets the subject prefix (default: workflow).
// WORKFLOW_NATS_EMBEDDED explicitly controls embedded NATS (auto-detected if not set).
// WORKFLOW_NATS_PORT sets the port for embedded NATS server (default: 4222).
func EnvOptions() Options {
	natsURL := getenvDefault("WORKFLOW_NATS_URL", defaultNATSURL)
	embeddedPort := getenvInt("WORKFLOW_NATS_PORT", 4222)

	// Auto-detect: if URL is explicitly set to non-default, disable embedded
	// User can still override with WORKFLOW_NATS_EMBEDDED=true/false
	embedded := true
	if natsURL != defaultNATSURL {
		embedded = false
	}
	if val := os.Getenv("WORKFLOW_NATS_EMBEDDED"); val != "" {
		embedded = getenvBool("WORKFLOW_NATS_EMBEDDED", embedded)
	}

	opts := Options{
		Enabled:        getenvBool("WORKFLOW_NATS_ENABLED", true),
		URL:            natsURL,
		Stream:         getenvDefault("WORKFLOW_NATS_STREAM", "WORKFLOW_EVENTS"),
		SubjectPrefix:  getenvDefault("WORKFLOW_NATS_SUBJECT_PREFIX", "workflow"),
		ConsumerName:   getenvDefault("WORKFLOW_NATS_CONSUMER", "station-workflow"),
		Embedded:       embedded,
		EmbeddedPort:   embeddedPort,
		WorkerPoolSize: getenvInt("WORKFLOW_WORKER_POOL_SIZE", 10),
	}
	return opts
}

func getenvDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}
