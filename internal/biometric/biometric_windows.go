//go:build windows

package biometric

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unicode/utf16"
)

const Name = "Windows Hello"

const powershellPrelude = `
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Security.Credentials.UI.UserConsentVerifier, Windows.Security.Credentials.UI, ContentType=WindowsRuntime]
function Await-WinRT($Operation, $ResultType) {
  $method = [System.WindowsRuntimeSystemExtensions].GetMethods() |
    Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 } |
    Select-Object -First 1
  $task = $method.MakeGenericMethod($ResultType).Invoke($null, @($Operation))
  $task.Wait()
  return $task.Result
}
`

func Available() bool {
	script := powershellPrelude + `
$result = Await-WinRT ([Windows.Security.Credentials.UI.UserConsentVerifier]::CheckAvailabilityAsync()) ([Windows.Security.Credentials.UI.UserConsentVerifierAvailability])
Write-Output $result.ToString()
`
	output, err := runPowerShell(script)
	return err == nil && output == "Available"
}

func Authenticate(reason string) error {
	script := powershellPrelude + fmt.Sprintf(`
$result = Await-WinRT ([Windows.Security.Credentials.UI.UserConsentVerifier]::RequestVerificationAsync('%s')) ([Windows.Security.Credentials.UI.UserConsentVerificationResult])
Write-Output $result.ToString()
`, escapePowerShell(reason))
	output, err := runPowerShell(script)
	if err != nil {
		return fmt.Errorf("Windows Hello failed: %w", err)
	}
	if output != "Verified" {
		return fmt.Errorf("Windows Hello verification was not completed: %s", output)
	}
	return nil
}

func runPowerShell(script string) (string, error) {
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		bytes[i*2] = byte(value)
		bytes[i*2+1] = byte(value >> 8)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(bytes))
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := command.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("verification timed out")
	}
	return trimOutput(string(output)), err
}

func escapePowerShell(value string) string {
	result := ""
	for _, char := range value {
		if char == '\'' {
			result += "''"
		} else {
			result += string(char)
		}
	}
	return result
}

func trimOutput(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\r' || value[len(value)-1] == '\n' || value[len(value)-1] == ' ') {
		value = value[:len(value)-1]
	}
	return value
}
