package grpcclient

import "testing"

func TestTransportConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		processOptIn  string
		allowInsecure bool
		valid         bool
	}{
		{
			name: "plaintext rejected without opt in",
		},
		{
			name:         "process environment cannot override explicit config",
			processOptIn: "true",
		},
		{
			name:          "explicit plaintext opt in",
			allowInsecure: true,
			valid:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ALLOW_INSECURE_GRPC", tt.processOptIn)
			t.Setenv("GRPC_CA_FILE", "")
			conn, err := Dial(Config{
				Address:       "127.0.0.1:1",
				ServiceName:   "test",
				AllowInsecure: tt.allowInsecure,
			})
			if conn != nil {
				t.Cleanup(func() {

					_ = conn.Close()
				})
			}
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}
