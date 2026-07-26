package credentials

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "SSHKing SSH passwords"

var ErrNotFound = errors.New("saved password not found")

func Set(serverID, password string) error {
	return keyring.Set(service, serverID, password)
}

func Get(serverID string) (string, error) {
	password, err := keyring.Get(service, serverID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return password, err
}

func Delete(serverID string) error {
	err := keyring.Delete(service, serverID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
