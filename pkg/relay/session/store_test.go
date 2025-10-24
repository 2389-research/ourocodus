package session

import (
	"testing"
)

// TestMemoryStore_CreateDuplicateID tests error on duplicate session ID
func TestMemoryStore_CreateDuplicateID(t *testing.T) {
	store := NewMemoryStore()
	session := NewUserSession("test-id", &mockWebSocket{}, testTime())

	// Create once - should succeed
	err := store.Create(session)
	if err != nil {
		t.Fatalf("Expected no error on first create, got: %v", err)
	}

	// Create again - should fail
	err = store.Create(session)
	if err == nil {
		t.Fatal("Expected error on duplicate create, got nil")
	}
}

// TestMemoryStore_CreateNil tests error on nil session
func TestMemoryStore_CreateNil(t *testing.T) {
	store := NewMemoryStore()
	err := store.Create(nil)
	if err == nil {
		t.Fatal("Expected error on nil session, got nil")
	}
}

// TestMemoryStore_DeleteNonexistent tests idempotent delete
func TestMemoryStore_DeleteNonexistent(t *testing.T) {
	store := NewMemoryStore()
	// Should not panic or error
	store.Delete("nonexistent")
}
