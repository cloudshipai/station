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
	WorkerPoolSize int // Number of concurrent workflow step processors (default: 10)
}

// EnvOptions builds runtime options from environment variables.
// WORKFLOW_NATS_ENABLED=true enables the engine.
// WORKFLOW_NATS_URL overrides the NATS connection string (default: nats://127.0.0.1:4222).
// WORKFLOW_NATS_STREAM sets the JetStream stream name (default: WORKFLOW_EVENTS).
// WORKFLOW_NATS_SUBJECT_PREFIX sets the subject prefix (default: workflow).
// WORKFLOW_NATS_EMBEDDED=true starts an embedded NATS server for local development.
func EnvOptions() Options {
	opts := Options{
		Enabled:        getenvBool("WORKFLOW_NATS_ENABLED", true),
		URL:            getenvDefault("WORKFLOW_NATS_URL", "nats://127.0.0.1:4222"),
		Stream:         getenvDefault("WORKFLOW_NATS_STREAM", "WORKFLOW_EVENTS"),
		SubjectPrefix:  getenvDefault("WORKFLOW_NATS_SUBJECT_PREFIX", "workflow"),
		ConsumerName:   getenvDefault("WORKFLOW_NATS_CONSUMER", "station-workflow"),
		Embedded:       getenvBool("WORKFLOW_NATS_EMBEDDED", true),
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
