package process

import "os"

// Executable returns the running program's path for starting a new instance.
func Executable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	return resolveExecutable(executable)
}
