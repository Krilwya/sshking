package config

import (
	"strings"
	"testing"
)

func TestTranscriptRoundTripAndClear(t *testing.T) {
	store := &Store{dir: t.TempDir()}
	if err := store.AppendTranscript("tab/../unsafe", "prompt$ ls\r\nresult\r\n"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Transcript("tab/../unsafe")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "result") {
		t.Fatalf("unexpected transcript: %q", got)
	}
	if err := store.ClearTranscript("tab/../unsafe"); err != nil {
		t.Fatal(err)
	}
	got, err = store.Transcript("tab/../unsafe")
	if err != nil || got != "" {
		t.Fatalf("transcript not cleared: %q, %v", got, err)
	}
}
