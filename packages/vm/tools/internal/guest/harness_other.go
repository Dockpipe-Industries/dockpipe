//go:build !linux

package guest

import "fmt"

func NewLinuxHarnessAdapter(string, string, string) (HarnessAdapter, error) {
	return nil, fmt.Errorf("Linux qualification harness is unavailable on this platform")
}
