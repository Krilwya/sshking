package editor

import (
	"testing"

	"sshking/internal/config"
)

func TestZedSSHURL(t *testing.T) {
	server := config.Server{Host: "example.com", Port: 2222, User: "deploy"}
	tests := map[string]string{
		"":                "ssh://deploy@example.com:2222/~",
		"~":               "ssh://deploy@example.com:2222/~",
		"~/project":       "ssh://deploy@example.com:2222/~/project",
		"project/file.go": "ssh://deploy@example.com:2222/~/project/file.go",
		"/srv/my project": "ssh://deploy@example.com:2222/srv/my%20project",
	}
	for input, want := range tests {
		if got := zedSSHURL(server, input, ""); got != want {
			t.Errorf("zedSSHURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestZedSSHURLEncodesPassword(t *testing.T) {
	server := config.Server{Host: "example.com", Port: 22, User: "deploy"}
	got := zedSSHURL(server, "~/project", "p@ss:/ word")
	want := "ssh://deploy:p%40ss%3A%2F%20word@example.com:22/~/project"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
