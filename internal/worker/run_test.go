package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nick-fedchik/codex-fleet/internal/config"
)

func TestTransportRunUsesRemoteOllamaResponse(t *testing.T) {
	dir := t.TempDir()
	fakeSSH := filepath.Join(dir, "ssh")
	if err := os.WriteFile(fakeSSH, []byte("#!/bin/sh\nprintf '%s' '{\"response\":\"worker online\",\"done\":true,\"load_duration\":123,\"total_duration\":456}'\n"), 0o755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := (Transport{Timeout: 2 * time.Second}).Run(context.Background(), config.Worker{SSHHost: "fake"}, "test-model", "hello", "10m")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Response != "worker online" {
		t.Fatalf("Run().Response = %q, want %q", result.Response, "worker online")
	}
	if result.LoadDuration != 123 || result.TotalDuration != 456 {
		t.Fatalf("Run() metrics = load %d total %d", result.LoadDuration, result.TotalDuration)
	}
}

func TestTransportRemoteTimeoutFollowsOperationTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    int
	}{
		{name: "configured", timeout: 31 * time.Minute, want: 1860},
		{name: "default", timeout: 0, want: 10},
		{name: "minimum", timeout: 500 * time.Millisecond, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Transport{Timeout: tt.timeout}).remoteTimeoutSeconds(); got != tt.want {
				t.Fatalf("remoteTimeoutSeconds() = %d, want %d", got, tt.want)
			}
		})
	}
}
