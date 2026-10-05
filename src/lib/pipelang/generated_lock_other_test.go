//go:build !linux

package pipelang

import "fmt"

func lockGeneratedArtifact(root, key string) (func(), error) {
	return nil, fmt.Errorf("compiled artifact reuse requires the Linux contained validation lane")
}
