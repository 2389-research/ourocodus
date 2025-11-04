package relay_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock NATS client for testing
type mockNATSClient struct {
	publishes  []mockPublish
	mu         sync.Mutex
	publishErr error
}

type mockPublish struct {
	subject string
	data    []byte
}

func (m *mockNATSClient) Publish(ctx context.Context, subject string, data []byte, opts ...nats.PubOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return m.publishErr
	}

	m.publishes = append(m.publishes, mockPublish{subject: subject, data: data})
	return nil
}

func (m *mockNATSClient) Subscribe(ctx context.Context, subject string, handler nats.MsgHandler, opts ...nats.SubOption) (*nats.Subscription, error) {
	return nil, nil
}

func (m *mockNATSClient) Request(ctx context.Context, subject string, data []byte, opts ...nats.ReqOption) (*nats.Message, error) {
	return nil, nil
}

func (m *mockNATSClient) JS() (nats.JSClient, error) {
	return nil, nil
}

func (m *mockNATSClient) Health() nats.HealthStatus {
	return nats.HealthStatus{Connected: true}
}

func (m *mockNATSClient) Ready() error {
	return nil
}

func (m *mockNATSClient) Drain(ctx context.Context) error {
	return nil
}

func (m *mockNATSClient) Close() error {
	return nil
}

func (m *mockNATSClient) Raw() *natsgo.Conn {
	return nil
}

// Helper to get publish history
func (m *mockNATSClient) getPublishes() []mockPublish {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockPublish{}, m.publishes...)
}

// Mock ID generator
type mockIDGenerator struct {
	id string
}

func (m *mockIDGenerator) Generate() string {
	return m.id
}

// Mock clock
type mockClock struct {
	now string
}

func (m *mockClock) Now() string {
	return m.now
}

// Mock logger
type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store for verification if needed
	m.messages = append(m.messages, format)
}

func TestNATSEventPublisher_SessionCreated(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-message-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	err := publisher.PublishSessionCreated(context.Background(), "test-session")
	require.NoError(t, err)

	// Verify publish was called
	publishes := mockClient.getPublishes()
	require.Len(t, publishes, 1)
	publish := publishes[0]

	// Verify subject
	assert.Equal(t, "sessions.test-session.session.created", publish.subject)

	// Verify payload structure
	var event map[string]interface{}
	err = json.Unmarshal(publish.data, &event)
	require.NoError(t, err)

	assert.Equal(t, "1.0", event["version"])
	assert.Equal(t, "test-message-id", event["messageId"])
	assert.Equal(t, float64(0), event["eventIndex"]) // First event
	assert.Equal(t, "2025-11-04T10:00:00Z", event["occurredAt"])
	assert.Equal(t, "2025-11-04T10:00:00Z", event["publishedAt"])
	assert.Equal(t, "session.created", event["type"])

	payload := event["payload"].(map[string]interface{})
	assert.Equal(t, "test-session", payload["userSessionId"])
	assert.Equal(t, "2025-11-04T10:00:00Z", payload["createdAt"])
}

func TestNATSEventPublisher_SessionTerminated(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "msg-123"}
	clock := &mockClock{now: "2025-11-04T10:05:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	err := publisher.PublishSessionTerminated(context.Background(), "session-456")
	require.NoError(t, err)

	publishes := mockClient.getPublishes()
	require.Len(t, publishes, 1)

	assert.Equal(t, "sessions.session-456.session.terminated", publishes[0].subject)

	var event map[string]interface{}
	json.Unmarshal(publishes[0].data, &event)

	assert.Equal(t, "session.terminated", event["type"])
	payload := event["payload"].(map[string]interface{})
	assert.Equal(t, "session-456", payload["userSessionId"])
	assert.Equal(t, "2025-11-04T10:05:00Z", payload["terminatedAt"])
}

func TestNATSEventPublisher_AgentSpawned(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "msg-789"}
	clock := &mockClock{now: "2025-11-04T10:10:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	err := publisher.PublishAgentSpawned(context.Background(), "session-abc", "coder", "/workspace/coder")
	require.NoError(t, err)

	publishes := mockClient.getPublishes()
	require.Len(t, publishes, 1)

	assert.Equal(t, "sessions.session-abc.agent.spawned", publishes[0].subject)

	var event map[string]interface{}
	json.Unmarshal(publishes[0].data, &event)

	assert.Equal(t, "agent.spawned", event["type"])
	payload := event["payload"].(map[string]interface{})
	assert.Equal(t, "session-abc", payload["userSessionId"])
	assert.Equal(t, "coder", payload["agentId"])
	assert.Equal(t, "/workspace/coder", payload["workspace"])
	assert.Equal(t, "2025-11-04T10:10:00Z", payload["spawnedAt"])
}

func TestNATSEventPublisher_AgentTerminated(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "msg-999"}
	clock := &mockClock{now: "2025-11-04T10:15:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	err := publisher.PublishAgentTerminated(context.Background(), "session-xyz", "tester", 0)
	require.NoError(t, err)

	publishes := mockClient.getPublishes()
	require.Len(t, publishes, 1)

	assert.Equal(t, "sessions.session-xyz.agent.terminated", publishes[0].subject)

	var event map[string]interface{}
	json.Unmarshal(publishes[0].data, &event)

	assert.Equal(t, "agent.terminated", event["type"])
	payload := event["payload"].(map[string]interface{})
	assert.Equal(t, "session-xyz", payload["userSessionId"])
	assert.Equal(t, "tester", payload["agentId"])
	assert.Equal(t, float64(0), payload["exitCode"])
	assert.Equal(t, "2025-11-04T10:15:00Z", payload["terminatedAt"])
}

func TestNATSEventPublisher_EventIndexIncrement(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Publish multiple events for same session
	publisher.PublishSessionCreated(context.Background(), "test-session")
	publisher.PublishAgentSpawned(context.Background(), "test-session", "coder", "/ws")
	publisher.PublishAgentTerminated(context.Background(), "test-session", "coder", 0)
	publisher.PublishSessionTerminated(context.Background(), "test-session")

	// Verify event indices increment
	publishes := mockClient.getPublishes()
	for i, publish := range publishes {
		var event map[string]interface{}
		json.Unmarshal(publish.data, &event)
		assert.Equal(t, float64(i), event["eventIndex"], "Event %d should have eventIndex %d", i, i)
	}
}

func TestNATSEventPublisher_EventIndexPerSession(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Publish events for different sessions
	publisher.PublishSessionCreated(context.Background(), "session-1")
	publisher.PublishSessionCreated(context.Background(), "session-2")
	publisher.PublishAgentSpawned(context.Background(), "session-1", "agent", "/ws")
	publisher.PublishAgentSpawned(context.Background(), "session-2", "agent", "/ws")

	// Verify each session has its own counter
	publishes := mockClient.getPublishes()

	// Session-1 events should have indices 0, 1
	var event1 map[string]interface{}
	json.Unmarshal(publishes[0].data, &event1)
	assert.Equal(t, float64(0), event1["eventIndex"])

	var event3 map[string]interface{}
	json.Unmarshal(publishes[2].data, &event3)
	assert.Equal(t, float64(1), event3["eventIndex"])

	// Session-2 events should have indices 0, 1
	var event2 map[string]interface{}
	json.Unmarshal(publishes[1].data, &event2)
	assert.Equal(t, float64(0), event2["eventIndex"])

	var event4 map[string]interface{}
	json.Unmarshal(publishes[3].data, &event4)
	assert.Equal(t, float64(1), event4["eventIndex"])
}

func TestNATSEventPublisher_PublishError(t *testing.T) {
	mockClient := &mockNATSClient{
		publishErr: errors.New("NATS connection lost"),
	}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	err := publisher.PublishSessionCreated(context.Background(), "test-session")

	// Error should be returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NATS connection lost")

	// Logger should have recorded error
	assert.NotEmpty(t, logger.messages)
}

func TestNATSEventPublisher_CorrelationID(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Create context with correlationId
	ctx := relay.WithCorrelationID(context.Background(), "request-123")

	err := publisher.PublishSessionCreated(ctx, "test-session")
	require.NoError(t, err)

	publishes := mockClient.getPublishes()
	var event map[string]interface{}
	json.Unmarshal(publishes[0].data, &event)

	// Verify correlationId is included
	assert.Equal(t, "request-123", event["correlationId"])
}

func TestNATSEventPublisher_Concurrent(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Publish concurrently from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			publisher.PublishSessionCreated(context.Background(), "concurrent-session")
		}()
	}

	wg.Wait()

	// Verify all 10 events were published
	publishes := mockClient.getPublishes()
	assert.Len(t, publishes, 10)

	// Verify event indices are sequential (0-9)
	indices := make(map[int64]bool)
	for _, publish := range publishes {
		var event map[string]interface{}
		json.Unmarshal(publish.data, &event)
		idx := int64(event["eventIndex"].(float64))
		indices[idx] = true
	}

	// All indices 0-9 should be present
	for i := int64(0); i < 10; i++ {
		assert.True(t, indices[i], "Index %d should be present", i)
	}
}

func TestNATSEventPublisher_CleanupSession(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Publish events to build up eventIndex counter
	publisher.PublishSessionCreated(context.Background(), "session-1")
	publisher.PublishAgentSpawned(context.Background(), "session-1", "agent", "/ws")

	// Verify eventIndex increments
	publishes := mockClient.getPublishes()
	var event1 map[string]interface{}
	json.Unmarshal(publishes[0].data, &event1)
	assert.Equal(t, float64(0), event1["eventIndex"])

	var event2 map[string]interface{}
	json.Unmarshal(publishes[1].data, &event2)
	assert.Equal(t, float64(1), event2["eventIndex"])

	// Clean up the session
	publisher.CleanupSession("session-1")

	// Publish new event for same session - should restart at 0
	publisher.PublishSessionCreated(context.Background(), "session-1")

	publishes = mockClient.getPublishes()
	var event3 map[string]interface{}
	json.Unmarshal(publishes[2].data, &event3)
	assert.Equal(t, float64(0), event3["eventIndex"], "Event index should restart at 0 after cleanup")
}

func TestNATSEventPublisher_SessionTerminatedCleansUp(t *testing.T) {
	mockClient := &mockNATSClient{}
	idGen := &mockIDGenerator{id: "test-id"}
	clock := &mockClock{now: "2025-11-04T10:00:00Z"}
	logger := &mockLogger{}

	publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

	// Publish events to build up eventIndex counter
	publisher.PublishSessionCreated(context.Background(), "session-1")
	publisher.PublishAgentSpawned(context.Background(), "session-1", "agent", "/ws")

	// Verify eventIndex increments
	publishes := mockClient.getPublishes()
	var event1 map[string]interface{}
	json.Unmarshal(publishes[0].data, &event1)
	assert.Equal(t, float64(0), event1["eventIndex"])

	var event2 map[string]interface{}
	json.Unmarshal(publishes[1].data, &event2)
	assert.Equal(t, float64(1), event2["eventIndex"])

	// Publish session.terminated - should auto-cleanup
	publisher.PublishSessionTerminated(context.Background(), "session-1")

	publishes = mockClient.getPublishes()
	var event3 map[string]interface{}
	json.Unmarshal(publishes[2].data, &event3)
	assert.Equal(t, float64(2), event3["eventIndex"], "Session terminated should have correct index")

	// Publish new event for same session - should restart at 0
	publisher.PublishSessionCreated(context.Background(), "session-1")

	publishes = mockClient.getPublishes()
	var event4 map[string]interface{}
	json.Unmarshal(publishes[3].data, &event4)
	assert.Equal(t, float64(0), event4["eventIndex"], "Event index should restart at 0 after session.terminated auto-cleanup")
}
