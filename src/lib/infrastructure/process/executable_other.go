//go:build !linux

package process

func resolveExecutable(executable string) (string, error) {
	return executable, nil
}
