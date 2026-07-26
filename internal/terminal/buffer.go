package terminal

import (
	"strings"
	"sync"
)

type Buffer struct {
	mu       sync.RWMutex
	lines    []string
	partial  string
	capacity int
	sanitize *Sanitizer
}

func NewBuffer(capacity int) *Buffer {
	if capacity < 100 {
		capacity = 100
	}
	return &Buffer{
		capacity: capacity,
		lines:    make([]string, 0, min(capacity, 256)),
		sanitize: NewSanitizer(),
	}
}

func (b *Buffer) Append(data string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	clean := b.sanitize.Write(data)
	chunks := strings.Split(b.partial+clean, "\n")
	b.partial = chunks[len(chunks)-1]
	for _, line := range chunks[:len(chunks)-1] {
		b.lines = append(b.lines, line)
	}
	if overflow := len(b.lines) - b.capacity; overflow > 0 {
		copy(b.lines, b.lines[overflow:])
		b.lines = b.lines[:b.capacity]
	}
}

func (b *Buffer) AddLine(line string) {
	b.Append(line + "\n")
}

func (b *Buffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]string, 0, len(b.lines)+1)
	result = append(result, b.lines...)
	if b.partial != "" {
		result = append(result, b.partial)
	}
	return result
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = b.lines[:0]
	b.partial = ""
	b.sanitize.Reset()
}
