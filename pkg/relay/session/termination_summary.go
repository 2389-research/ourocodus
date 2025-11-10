package session

// CleanupStatus represents the result of relay cleanup operations once a session ends.
type CleanupStatus string

const (
	// CleanupStatusComplete indicates all termination and cleanup steps succeeded.
	CleanupStatusComplete CleanupStatus = "complete"
	// CleanupStatusPartial indicates some steps failed but core termination continued.
	CleanupStatusPartial CleanupStatus = "partial"
	// CleanupStatusFailed indicates termination encountered critical failures.
	CleanupStatusFailed CleanupStatus = "failed"
)

// TerminationSummary captures telemetry about how user session shutdown progressed.
type TerminationSummary struct {
	AgentsTerminated int
	AgentFailures    int
	CleanupStatus    CleanupStatus
	Errors           []string
}

// addError appends an error to the summary (lazy allocation helper).
func (s *TerminationSummary) addError(msg string) {
	s.Errors = append(s.Errors, msg)
}
