package cloudclient

import "testing"

func TestNormalizeBase(t *testing.T) {
	valid := []string{"https://cloud.example.com", "http://127.0.0.1:8787", "http://localhost:8787/"}
	for _, v := range valid {
		if _, err := normalizeBase(v); err != nil {
			t.Errorf("%s: %v", v, err)
		}
	}
	invalid := []string{"http://cloud.example.com", "https://cloud.example.com/path", "ftp://cloud.example.com"}
	for _, v := range invalid {
		if _, err := normalizeBase(v); err == nil {
			t.Errorf("expected %s to fail", v)
		}
	}
}
