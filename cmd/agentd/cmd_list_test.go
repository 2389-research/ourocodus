package main

import (
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/cli/format"
)

func TestFormatContainerID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want int // expected length
	}{
		{
			name: "long container ID",
			id:   "abc123def456ghi789jkl012mno345",
			want: 12,
		},
		{
			name: "short container ID",
			id:   "abc123",
			want: 6,
		},
		{
			name: "exactly 12 characters",
			id:   "abc123def456",
			want: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.FormatContainerID(tt.id)
			if len(got) != tt.want {
				t.Errorf("FormatContainerID() length = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFormatPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxWidth int
		wantLen  int
	}{
		{
			name:     "short path unchanged",
			path:     "/short/path",
			maxWidth: 60,
			wantLen:  11,
		},
		{
			name:     "long path truncated",
			path:     "/very/long/path/that/exceeds/sixty/characters/and/should/be/truncated/significantly",
			maxWidth: 60,
			wantLen:  60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.FormatPath(tt.path, tt.maxWidth)
			if len(got) != tt.wantLen {
				t.Errorf("FormatPath() length = %d, want %d", len(got), tt.wantLen)
			}
			if len(tt.path) > tt.maxWidth && got[:3] != "..." {
				t.Errorf("FormatPath() long path = %q, should start with '...'", got)
			}
		})
	}
}

func TestFormatDurationHuman(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "under a minute shows just now",
			duration: 30 * time.Second,
			want:     "just now",
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
			want:     "5m ago",
		},
		{
			name:     "hours",
			duration: 3 * time.Hour,
			want:     "3h ago",
		},
		{
			name:     "days",
			duration: 2 * 24 * time.Hour,
			want:     "2d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.FormatDurationHuman(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDurationHuman(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
