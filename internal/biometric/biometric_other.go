//go:build !windows && !darwin

package biometric

import "errors"

const Name = "Device authentication"

func Available() bool {
	return false
}

func Authenticate(string) error {
	return errors.New("device authentication is unavailable")
}
