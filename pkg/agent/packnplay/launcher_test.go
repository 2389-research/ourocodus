package packnplay

import (
	"testing"

	"github.com/obra/packnplay/pkg/config"
)

func TestNewLauncher(t *testing.T) {
	tests := []struct {
		name    string
		opts    []LauncherOption
		wantErr bool
		check   func(*testing.T, *PacknplayLauncher)
	}{
		{
			name: "default options",
			opts: []LauncherOption{
				WithProjectPath("."),
			},
			wantErr: false,
			check: func(t *testing.T, l *PacknplayLauncher) {
				if l.defaultImage != "ubuntu:22.04" {
					t.Errorf("expected default image ubuntu:22.04, got %s", l.defaultImage)
				}
				if l.runtime != "docker" {
					t.Errorf("expected runtime docker, got %s", l.runtime)
				}
				if l.verbose {
					t.Error("expected verbose false")
				}
			},
		},
		{
			name: "custom image and runtime",
			opts: []LauncherOption{
				WithProjectPath("."),
				WithDefaultImage("alpine:latest"),
				WithRuntime("podman"),
				WithVerbose(true),
			},
			wantErr: false,
			check: func(t *testing.T, l *PacknplayLauncher) {
				if l.defaultImage != "alpine:latest" {
					t.Errorf("expected image alpine:latest, got %s", l.defaultImage)
				}
				if l.runtime != "podman" {
					t.Errorf("expected runtime podman, got %s", l.runtime)
				}
				if !l.verbose {
					t.Error("expected verbose true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLauncher(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewLauncher() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, l)
			}
			if err == nil && l != nil {
				_ = l.Close()
			}
		})
	}
}

func TestMapToEnvSlice(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want map[string]bool // Use map for set membership
	}{
		{
			name: "nil map",
			m:    nil,
			want: map[string]bool{},
		},
		{
			name: "empty map",
			m:    map[string]string{},
			want: map[string]bool{},
		},
		{
			name: "single entry",
			m:    map[string]string{"KEY": "value"},
			want: map[string]bool{"KEY=value": true},
		},
		{
			name: "multiple entries",
			m: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
			want: map[string]bool{
				"FOO=bar": true,
				"BAZ=qux": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapToEnvSlice(tt.m)
			if len(got) != len(tt.want) {
				t.Errorf("mapToEnvSlice() got %d entries, want %d", len(got), len(tt.want))
			}
			for _, entry := range got {
				if !tt.want[entry] {
					t.Errorf("mapToEnvSlice() got unexpected entry %q", entry)
				}
			}
		})
	}
}

func TestMapSpawnConfigCredentials(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want config.Credentials
	}{
		{
			name: "nil map - defaults",
			m:    nil,
			want: config.Credentials{
				Git: true,
				SSH: true,
				GH:  true,
				GPG: false,
				NPM: false,
			},
		},
		{
			name: "empty map - defaults",
			m:    map[string]string{},
			want: config.Credentials{
				Git: true,
				SSH: true,
				GH:  true,
				GPG: false,
				NPM: false,
			},
		},
		{
			name: "all enabled",
			m: map[string]string{
				"git": "true",
				"ssh": "true",
				"gh":  "true",
				"gpg": "true",
				"npm": "true",
			},
			want: config.Credentials{
				Git: true,
				SSH: true,
				GH:  true,
				GPG: true,
				NPM: true,
			},
		},
		{
			name: "github alias",
			m: map[string]string{
				"github": "true",
			},
			want: config.Credentials{
				Git: false,
				SSH: false,
				GH:  true,
				GPG: false,
				NPM: false,
			},
		},
		{
			name: "selective enable",
			m: map[string]string{
				"git": "true",
				"gpg": "true",
			},
			want: config.Credentials{
				Git: true,
				SSH: false,
				GH:  false,
				GPG: true,
				NPM: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSpawnConfigCredentials(tt.m)
			if got != tt.want {
				t.Errorf("mapSpawnConfigCredentials() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFindRunningAgents(t *testing.T) {
	// This test requires a Docker daemon, so we skip it in normal unit tests
	// It will be tested in integration tests
	t.Skip("FindRunningAgents requires Docker - tested in integration")
}

func TestPipeCloser(t *testing.T) {
	t.Run("basic pipe operations", func(t *testing.T) {
		p := newPipeCloser()
		defer p.Close()

		// Write to pipe
		go func() {
			_, _ = p.Writer().Write([]byte("hello"))
			_ = p.Writer().Close()
		}()

		// Read from pipe
		buf := make([]byte, 10)
		n, err := p.Reader().Read(buf)
		if err != nil {
			t.Fatalf("failed to read from pipe: %v", err)
		}

		if got := string(buf[:n]); got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("double close is safe", func(t *testing.T) {
		p := newPipeCloser()
		if err := p.Close(); err != nil {
			t.Errorf("first close failed: %v", err)
		}
		if err := p.Close(); err != nil {
			t.Error("second close should not error")
		}
	})
}

func TestWorktreeNameGeneration(t *testing.T) {
	// Test that agent IDs are properly formatted for worktree names
	// Verify worktree name format would be: agent-{ULID}
	// This is implicit in the Spawn implementation
	// Actual testing happens in integration tests where we spawn real containers

	// This is a placeholder test - the actual verification happens in integration tests
	// where we can inspect real worktree names created by Packnplay
}
