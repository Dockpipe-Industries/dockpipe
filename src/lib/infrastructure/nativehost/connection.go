package nativehost

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Connection struct {
	Schema  string `json:"schema"`
	Socket  string `json:"socket"`
	KeyFile string `json:"key_file"`
}

func validToken(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func readBoundedFile(path string, maximum int64) ([]byte, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, errors.New("native host reference must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, nil, errors.New("invalid native host reference file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, nil, errors.New("native host reference changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, nil, errors.New("native host reference too large")
	}
	return data, opened, nil
}

func ReadConnection(path string) (Connection, error) {
	var connection Connection
	data, _, err := readBoundedFile(path, 4096)
	if err != nil {
		return connection, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&connection) != nil || decoder.Decode(new(any)) != io.EOF || connection.Schema != ConnectionSchema || !filepath.IsAbs(connection.Socket) || filepath.Clean(connection.Socket) != connection.Socket || len(connection.Socket) > 107 || !filepath.IsAbs(connection.KeyFile) {
		return Connection{}, errors.New("invalid native host connection")
	}
	return connection, nil
}

func readKey(path string) (string, os.FileInfo, error) {
	data, info, err := readBoundedFile(path, 128)
	if err != nil {
		return "", nil, err
	}
	key := strings.TrimSpace(string(data))
	if info.Mode().Perm()&0077 != 0 || !validToken(key) {
		return "", nil, errors.New("invalid native host key file")
	}
	return key, info, nil
}
