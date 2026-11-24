//go:build !windows

package process

func hostBootSequence() (uint32, error) {
	return 0, nil
}
