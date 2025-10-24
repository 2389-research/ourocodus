package session

import (
	"testing"
	"time"
)

// TestNewManager_PanicsOnNilDependencies tests that NewManager panics with nil dependencies
func TestNewManager_PanicsOnNilDependencies(t *testing.T) {
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test"}
	clock := &mockClock{}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{}

	tests := []struct {
		name          string
		store         Store
		idGen         IDGenerator
		clock         Clock
		cleaner       Cleaner
		logger        Logger
		clientFactory ClientFactory
	}{
		{"nil store", nil, idGen, clock, cleaner, logger, clientFactory},
		{"nil idGen", store, nil, clock, cleaner, logger, clientFactory},
		{"nil clock", store, idGen, nil, cleaner, logger, clientFactory},
		{"nil cleaner", store, idGen, clock, nil, logger, clientFactory},
		{"nil logger", store, idGen, clock, cleaner, nil, clientFactory},
		{"nil clientFactory", store, idGen, clock, cleaner, logger, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected panic for %s, but didn't panic", tt.name)
				}
			}()
			NewManager(tt.store, tt.idGen, tt.clock, tt.cleaner, tt.logger, tt.clientFactory)
		})
	}
}

// testTime returns a fixed time for testing
func testTime() time.Time {
	return time.Date(2025, 10, 24, 12, 0, 0, 0, time.UTC)
}
