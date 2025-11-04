package containersession

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/filters"
)

const (
	// LabelPrefix is the namespace for all containersession labels
	LabelPrefix = "com.ourocodus.containersession"

	// LabelSessionID identifies the session ID
	LabelSessionID = "com.ourocodus.containersession.id"

	// LabelCreatedAt stores the creation timestamp
	LabelCreatedAt = "com.ourocodus.containersession.created"

	// LabelManagedBy identifies the manager
	LabelManagedBy = "com.ourocodus.containersession.managed-by"
)

// BuildLabels creates standard labels for a container session
func BuildLabels(sessionID string, timestamp time.Time) map[string]string {
	return map[string]string{
		LabelSessionID: sessionID,
		LabelCreatedAt: timestamp.Format(time.RFC3339),
		LabelManagedBy: "ourocodus-containersession",
	}
}

// BuildLabelFilters creates Docker API filters for finding sessions
func BuildLabelFilters(sessionID string) filters.Args {
	f := filters.NewArgs()
	if sessionID != "" {
		f.Add("label", fmt.Sprintf("%s=%s", LabelSessionID, sessionID))
	} else {
		// Find all containers managed by us
		f.Add("label", fmt.Sprintf("%s=ourocodus-containersession", LabelManagedBy))
	}
	return f
}
