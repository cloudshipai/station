package heartbeat

import "strings"

// Response tokens for heartbeat and memory operations
const (
	// HeartbeatOKToken indicates the heartbeat check found nothing to report
	HeartbeatOKToken = "HEARTBEAT_OK"

	// NoReplyToken indicates a memory flush with no user response needed
	NoReplyToken = "NO_REPLY"
)

// IsHeartbeatOK checks if a response indicates a successful heartbeat with nothing to report
func IsHeartbeatOK(response string) bool {
	normalized := strings.TrimSpace(strings.ToUpper(response))
	return normalized == HeartbeatOKToken || strings.Contains(normalized, HeartbeatOKToken)
}

// IsNoReply checks if a response indicates no user reply is needed
func IsNoReply(response string) bool {
	normalized := strings.TrimSpace(strings.ToUpper(response))
	return normalized == NoReplyToken || strings.Contains(normalized, NoReplyToken)
}

// ShouldNotify returns true if the heartbeat response requires notification
func ShouldNotify(response string) bool {
	return !IsHeartbeatOK(response) && !IsNoReply(response) && strings.TrimSpace(response) != ""
}
