package sshclient

import "testing"

func TestKeySlug(t *testing.T) {
	tests := map[string]string{
		"Production API":  "production_api",
		"  My---Server  ": "my_server",
		"":                "server",
	}
	for input, want := range tests {
		if got := keySlug(input); got != want {
			t.Errorf("keySlug(%q) = %q, want %q", input, got, want)
		}
	}
}
