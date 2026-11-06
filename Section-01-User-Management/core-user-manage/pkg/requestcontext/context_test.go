package requestcontext

import (
	"context"
	"strings"
	"testing"
)

func TestRequestIDs(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		preserve bool
	}{
		{"valid", "checkout-123_A.b", true}, {"maximum", strings.Repeat("a", 128), true},
		{"missing", "", false}, {"too long", strings.Repeat("a", 129), false},
		{"newline", "bad\nforged", false}, {"comma combined", "first,second", false}, {"unicode", "เลขคำขอ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := New(context.Background(), tt.id)
			if err != nil {
				t.Fatal(err)
			}
			got := ID(ctx)
			if !Valid(got) {
				t.Fatal("invalid generated ID")
			}
			if tt.preserve && got != tt.id {
				t.Fatal("valid ID not preserved")
			}
			if !tt.preserve && (got == tt.id || len(got) != 32) {
				t.Fatal("invalid ID not replaced")
			}
			if ID(context.Background()) != "" {
				t.Fatal("ID leaked across contexts")
			}
		})
	}
}
