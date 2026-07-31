package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ImportOpenSSH reads concrete Host entries from an OpenSSH config. Wildcard
// blocks are intentionally ignored because they describe defaults, not servers.
func ImportOpenSSH(path string, defaults Preferences) ([]Server, error) {
	if strings.TrimSpace(path) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".ssh", "config")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var servers []Server
	var current []Server
	flush := func() {
		for _, server := range current {
			if server.Host == "" {
				server.Host = server.Name
			}
			if server.User == "" {
				server.User = defaults.DefaultUser
			}
			if server.Port == 0 {
				server.Port = defaults.DefaultPort
			}
			if server.Shell == "" {
				server.Shell = defaults.DefaultShell
			}
			servers = append(servers, server)
		}
		current = nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.TrimSpace(strings.Join(fields[1:], " "))
		if key == "host" {
			flush()
			for _, alias := range fields[1:] {
				if strings.ContainsAny(alias, "*?!") {
					continue
				}
				current = append(current, Server{Name: alias, Group: "Imported"})
			}
			continue
		}
		for i := range current {
			switch key {
			case "hostname":
				current[i].Host = value
			case "user":
				current[i].User = value
			case "port":
				if port, parseErr := strconv.Atoi(value); parseErr == nil {
					current[i].Port = port
				}
			case "identityfile":
				if current[i].Identity == "" {
					current[i].Identity = value
				}
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}
