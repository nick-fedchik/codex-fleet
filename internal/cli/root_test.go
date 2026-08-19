package cli

import "testing"

func TestParseWorkerTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		user    string
		host    string
		wantErr bool
	}{
		{name: "user and IPv4", in: "mfedchyk@192.168.68.53", user: "mfedchyk", host: "192.168.68.53"},
		{name: "host only", in: "worker.local", host: "worker.local"},
		{name: "user and IPv6", in: "worker@[fe80::1]", user: "worker", host: "[fe80::1]"},
		{name: "missing user", in: "@192.168.68.53", wantErr: true},
		{name: "missing host", in: "mfedchyk@", wantErr: true},
		{name: "whitespace", in: "mfedchyk@192.168.68.53 ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, err := parseWorkerTarget(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseWorkerTarget(%q) succeeded, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWorkerTarget(%q) failed: %v", tt.in, err)
			}
			if user != tt.user || host != tt.host {
				t.Fatalf("parseWorkerTarget(%q) = user %q, host %q; want user %q, host %q", tt.in, user, host, tt.user, tt.host)
			}
		})
	}
}
