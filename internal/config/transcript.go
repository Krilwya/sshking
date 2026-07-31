package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

const maxTranscriptBytes = 4 << 20

func (s *Store) AppendTranscript(sessionID, data string) error {
	if sessionID == "" || data == "" {
		return nil
	}
	dir := filepath.Join(s.dir, "transcripts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, transcriptName(sessionID))
	if info, err := os.Stat(path); err == nil && info.Size()+int64(len(data)) > maxTranscriptBytes {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		keep := maxTranscriptBytes - len(data)
		if keep < 0 {
			data = data[len(data)-maxTranscriptBytes:]
			keep = 0
		}
		if len(existing) > keep {
			existing = existing[len(existing)-keep:]
		}
		if err := os.WriteFile(path, append(existing, data...), 0o600); err != nil {
			return err
		}
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(data)
	return err
}

func (s *Store) Transcript(sessionID string) (string, error) {
	if sessionID == "" {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(s.dir, "transcripts", transcriptName(sessionID)))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(data) > maxTranscriptBytes {
		data = data[len(data)-maxTranscriptBytes:]
	}
	return string(data), nil
}

func (s *Store) ClearTranscript(sessionID string) error {
	err := os.Remove(filepath.Join(s.dir, "transcripts", transcriptName(sessionID)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Store) ClearTranscripts() error {
	err := os.RemoveAll(filepath.Join(s.dir, "transcripts"))
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(s.dir, "transcripts"), 0o700)
}

func transcriptName(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return fmt.Sprintf("%x.ansi", sum[:16])
}
