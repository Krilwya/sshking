package terminal

import "strings"

type sanitizerState uint8

const (
	sanitizerText sanitizerState = iota
	sanitizerEscape
	sanitizerEscapeIntermediate
	sanitizerCSI
	sanitizerOSC
	sanitizerOSCEscape
)

// Sanitizer removes ANSI/VT control sequences from a terminal byte stream.
// Its parsing state is retained between calls because SSH reads may split a
// sequence at any byte.
type Sanitizer struct {
	state sanitizerState
}

func NewSanitizer() *Sanitizer {
	return &Sanitizer{}
}

func (s *Sanitizer) Write(value string) string {
	var result strings.Builder
	result.Grow(len(value))

	for i := 0; i < len(value); i++ {
		ch := value[i]

		switch s.state {
		case sanitizerText:
			switch ch {
			case 0x1b:
				s.state = sanitizerEscape
			case '\r', '\b', 0x00:
				// The DOM output is append-only, so cursor movement controls
				// must not be rendered as visible replacement characters.
			default:
				if ch >= 0x20 || ch == '\n' || ch == '\t' {
					result.WriteByte(ch)
				}
			}

		case sanitizerEscape:
			switch ch {
			case '[':
				s.state = sanitizerCSI
			case ']':
				s.state = sanitizerOSC
			default:
				if ch >= 0x20 && ch <= 0x2f {
					s.state = sanitizerEscapeIntermediate
				} else {
					s.state = sanitizerText
				}
			}

		case sanitizerEscapeIntermediate:
			if ch >= 0x30 && ch <= 0x7e {
				s.state = sanitizerText
			}

		case sanitizerCSI:
			if ch >= 0x40 && ch <= 0x7e {
				s.state = sanitizerText
			}

		case sanitizerOSC:
			switch ch {
			case '\a':
				s.state = sanitizerText
			case 0x1b:
				s.state = sanitizerOSCEscape
			}

		case sanitizerOSCEscape:
			if ch == '\\' {
				s.state = sanitizerText
			} else if ch != 0x1b {
				s.state = sanitizerOSC
			}
		}
	}

	return result.String()
}

func (s *Sanitizer) Reset() {
	s.state = sanitizerText
}
