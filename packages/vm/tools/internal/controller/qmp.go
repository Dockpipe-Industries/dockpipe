package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
)

const maxQMPMessage = 64 * 1024

var safeQMPCommands = map[string]struct{}{"query-status": {}, "query-uuid": {}}

type QMPGreeting struct {
	QMP struct {
		Version      map[string]any `json:"version"`
		Capabilities []string       `json:"capabilities"`
	} `json:"QMP"`
}

type QMPResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error,omitempty"`
	ID string `json:"id,omitempty"`
}

func ReadGreeting(r io.Reader) (QMPGreeting, error) {
	var greeting QMPGreeting
	line, err := readBoundedLine(r)
	if err != nil {
		return greeting, err
	}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	if err := dec.Decode(&greeting); err != nil || greeting.QMP.Version == nil {
		return greeting, fmt.Errorf("invalid QMP greeting")
	}
	return greeting, nil
}

type QMPClient struct{ Conn net.Conn }

func (c QMPClient) Negotiate(id string) error {
	if c.Conn == nil {
		return fmt.Errorf("QMP connection is required")
	}
	if _, err := ReadGreeting(c.Conn); err != nil {
		return err
	}
	_, err := c.executeExact("qmp_capabilities", id)
	return err
}

func (c QMPClient) SystemPowerdown(id string) error {
	_, err := c.executeExact("system_powerdown", id)
	return err
}

func (c QMPClient) Query(command, id string) (QMPResponse, error) {
	var response QMPResponse
	if c.Conn == nil {
		return response, fmt.Errorf("QMP connection is required")
	}
	if _, ok := safeQMPCommands[command]; !ok {
		return response, fmt.Errorf("QMP command %q is not read-only", command)
	}
	return c.executeExact(command, id)
}

func (c QMPClient) executeExact(command, id string) (QMPResponse, error) {
	var response QMPResponse
	if c.Conn == nil {
		return response, fmt.Errorf("QMP connection is required")
	}
	if command != "qmp_capabilities" && command != "system_powerdown" {
		if _, ok := safeQMPCommands[command]; !ok {
			return response, fmt.Errorf("QMP command %q is not permitted", command)
		}
	}
	request, _ := json.Marshal(struct {
		Execute string `json:"execute"`
		ID      string `json:"id"`
	}{Execute: command, ID: id})
	request = append(request, '\n')
	if _, err := c.Conn.Write(request); err != nil {
		return response, err
	}
	line, err := readBoundedLine(c.Conn)
	if err != nil {
		return response, err
	}
	if err := json.Unmarshal(line, &response); err != nil {
		return response, fmt.Errorf("decode QMP response: %w", err)
	}
	if response.ID != id {
		return response, fmt.Errorf("QMP response id mismatch")
	}
	if response.Error != nil {
		return response, fmt.Errorf("QMP %s: %s", response.Error.Class, response.Error.Desc)
	}
	return response, nil
}

func readBoundedLine(r io.Reader) ([]byte, error) {
	line := make([]byte, 0, 256)
	var one [1]byte
	for len(line) <= maxQMPMessage {
		if _, err := io.ReadFull(r, one[:]); err != nil {
			return nil, err
		}
		line = append(line, one[0])
		if one[0] == '\n' {
			return line, nil
		}
	}
	return nil, fmt.Errorf("QMP message exceeds %d bytes", maxQMPMessage)
}
