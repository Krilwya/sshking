//go:build darwin

package biometric

/*
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication
#include <stdlib.h>

int sshking_biometric_available(void);
int sshking_biometric_authenticate(const char *reason, char **error_message);
*/
import "C"

import (
	"errors"
	"unsafe"
)

const Name = "Touch ID"

func Available() bool {
	return C.sshking_biometric_available() == 1
}

func Authenticate(reason string) error {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	var message *C.char
	if C.sshking_biometric_authenticate(cReason, &message) == 1 {
		return nil
	}
	if message == nil {
		return errors.New("Touch ID verification failed")
	}
	defer C.free(unsafe.Pointer(message))
	return errors.New(C.GoString(message))
}
