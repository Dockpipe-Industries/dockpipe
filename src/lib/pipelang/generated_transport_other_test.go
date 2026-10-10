//go:build !linux

package pipelang

import "os"

func acquireGeneratedRepresentation(root, key string) (*os.File, error) { return nil, nil }
