package terminal

import (
	"reflect"
	"testing"
)

func TestBufferBoundsAndStripsANSI(t *testing.T) {
	buffer := NewBuffer(100)
	for i := 0; i < 105; i++ {
		buffer.AddLine("\x1b[32mok\x1b[0m")
	}
	lines := buffer.Lines()
	if len(lines) != 100 {
		t.Fatalf("expected 100 bounded lines, got %d", len(lines))
	}
	if lines[0] != "ok" {
		t.Fatalf("expected ANSI-free output, got %q", lines[0])
	}
}

func TestBufferJoinsPartialWrites(t *testing.T) {
	buffer := NewBuffer(100)
	buffer.Append("hel")
	buffer.Append("lo\nworld")
	if got, want := buffer.Lines(), []string{"hello", "world"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestSanitizerHandlesSplitCSISequences(t *testing.T) {
	sanitizer := NewSanitizer()
	if got := sanitizer.Write("\x1b[01;"); got != "" {
		t.Fatalf("expected incomplete sequence to produce no output, got %q", got)
	}
	if got, want := sanitizer.Write("34mblue\x1b[0m"), "blue"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizerRemovesBracketedPasteControls(t *testing.T) {
	sanitizer := NewSanitizer()
	got := sanitizer.Write("\x1b[?2004lubuntu\x1b[?2004h")
	if want := "ubuntu"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizerHandlesSplitOSCSequences(t *testing.T) {
	sanitizer := NewSanitizer()
	if got := sanitizer.Write("\x1b]0;user@host"); got != "" {
		t.Fatalf("expected incomplete OSC to produce no output, got %q", got)
	}
	if got, want := sanitizer.Write("\x1b\\prompt"), "prompt"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizerPreservesUnicode(t *testing.T) {
	sanitizer := NewSanitizer()
	if got, want := sanitizer.Write("héllo 世界\n"), "héllo 世界\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBufferHandlesANSISequenceSplitAcrossWrites(t *testing.T) {
	buffer := NewBuffer(100)
	buffer.Append("\x1b[01;")
	buffer.Append("34mserver\x1b[0m\n")
	if got, want := buffer.Lines(), []string{"server"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
