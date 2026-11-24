package infrastructure

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"dockpipe/src/lib/domain/workflow"
)

const controlFrameLimit = 1024 * 1024

type controlWireWriter struct {
	bytes.Buffer
	failure error
}

func (writer *controlWireWriter) append(data []byte) {
	if writer.failure != nil {
		return
	}
	if len(data) > controlFrameLimit-writer.Len() {
		writer.failure = fmt.Errorf("control frame exceeds its byte limit")
		return
	}
	_, _ = writer.Buffer.Write(data)
}
func (writer *controlWireWriter) byte(value byte) { writer.append([]byte{value}) }
func (writer *controlWireWriter) number(value uint64, width int) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	writer.append(encoded[:width])
}
func (writer *controlWireWriter) text(value string) {
	if len(value) > 65536 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		writer.failure = fmt.Errorf("invalid control protocol text")
		return
	}
	writer.number(uint64(len(value)), 4)
	writer.append([]byte(value))
}
func (writer *controlWireWriter) send(target io.Writer) error {
	if writer.failure != nil {
		return writer.failure
	}
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(writer.Len()))
	if _, err := io.Copy(target, bytes.NewReader(length[:])); err != nil {
		return err
	}
	_, err := io.Copy(target, bytes.NewReader(writer.Bytes()))
	return err
}

type controlWireReader struct{ *bytes.Reader }

func readControlFrame(source io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(source, length[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(length[:])
	if size == 0 || size > controlFrameLimit {
		return nil, fmt.Errorf("control frame exceeds its byte limit")
	}
	value := make([]byte, size)
	_, err := io.ReadFull(source, value)
	return value, err
}
func (reader controlWireReader) number(width int) (uint64, error) {
	var value [8]byte
	if _, err := io.ReadFull(reader.Reader, value[:width]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value[:]), nil
}
func (reader controlWireReader) text(limit uint64) (string, error) {
	size, err := reader.number(4)
	if err != nil {
		return "", err
	}
	if size > limit || size > uint64(reader.Len()) {
		return "", fmt.Errorf("invalid control protocol string length")
	}
	value := make([]byte, int(size))
	if _, err := io.ReadFull(reader.Reader, value); err != nil {
		return "", err
	}
	if !utf8.Valid(value) || bytes.ContainsRune(value, 0) {
		return "", fmt.Errorf("invalid control protocol text")
	}
	return string(value), nil
}

func encodeControlInputs(control *workflow.WorkflowControl, environment map[string]string) (*controlWireWriter, error) {
	writer := &controlWireWriter{}
	writer.byte(0)
	writer.number(1, 4)
	names := controlInputNames(control)
	writer.number(uint64(len(names)), 4)
	for _, name := range names {
		input := control.Inputs[name]
		value := input.Value
		if input.Env != "" {
			raw, exists := environment[input.Env]
			if !exists || len(raw) > 65536 {
				return nil, fmt.Errorf("control input %s requires a bounded explicit environment binding", name)
			}
			switch input.Type {
			case "string":
				value = raw
			case "bool":
				if raw != "true" && raw != "false" {
					return nil, fmt.Errorf("control input %s requires true or false", name)
				}
				value = raw == "true"
			case "int":
				parsed, err := strconv.ParseInt(raw, 10, 64)
				if err != nil || strconv.FormatInt(parsed, 10) != raw {
					return nil, fmt.Errorf("control input %s requires a canonical signed integer", name)
				}
				value = parsed
			case "List<string>":
				var values []string
				if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
					return nil, fmt.Errorf("control input %s requires a JSON string list", name)
				}
				value = values
			}
		}
		switch input.Type {
		case "bool":
			boolean, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("control input %s has the wrong type", name)
			}
			writer.byte(1)
			if boolean {
				writer.byte(1)
			} else {
				writer.byte(0)
			}
		case "int":
			var number int64
			switch typed := value.(type) {
			case int:
				number = int64(typed)
			case int64:
				number = typed
			default:
				return nil, fmt.Errorf("control input %s has the wrong integer type", name)
			}
			writer.byte(2)
			writer.number(uint64(number), 8)
		case "string":
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("control input %s has the wrong type", name)
			}
			writer.byte(3)
			writer.text(text)
		case "List<string>":
			var values []string
			switch typed := value.(type) {
			case []string:
				values = typed
			case []any:
				for _, item := range typed {
					text, ok := item.(string)
					if !ok {
						return nil, fmt.Errorf("control input %s requires strings", name)
					}
					values = append(values, text)
				}
			default:
				return nil, fmt.Errorf("control input %s requires a string list", name)
			}
			if len(values) > 256 {
				return nil, fmt.Errorf("control input %s exceeds its list bound", name)
			}
			writer.byte(4)
			writer.number(uint64(len(values)), 4)
			for _, text := range values {
				writer.text(text)
			}
		default:
			return nil, fmt.Errorf("unsupported control input type")
		}
		if writer.failure != nil {
			return nil, writer.failure
		}
	}
	return writer, nil
}

type controlMessage struct {
	kind       byte
	sequence   uint64
	step       string
	arguments  map[string]string
	outcome    byte
	diagnostic string
}

func decodeControlMessage(payload []byte) (controlMessage, error) {
	var message controlMessage
	reader := controlWireReader{bytes.NewReader(payload)}
	kind, err := reader.ReadByte()
	if err != nil {
		return message, err
	}
	message.kind = kind
	switch kind {
	case 1:
		message.sequence, err = reader.number(8)
		if err != nil {
			return message, err
		}
		message.step, err = reader.text(64)
		if err != nil {
			return message, err
		}
		count, err := reader.number(4)
		if err != nil || count > 16 {
			return message, fmt.Errorf("invalid control argument count")
		}
		message.arguments = map[string]string{}
		for index := uint64(0); index < count; index++ {
			name, err := reader.text(64)
			if err != nil {
				return message, err
			}
			value, err := reader.text(65536)
			if err != nil {
				return message, err
			}
			if _, exists := message.arguments[name]; exists {
				return message, fmt.Errorf("duplicate control argument")
			}
			message.arguments[name] = value
		}
	case 3:
		message.outcome, err = reader.ReadByte()
		if err != nil || message.outcome > 2 {
			return message, fmt.Errorf("invalid control outcome")
		}
		message.diagnostic, err = reader.text(4096)
		if err != nil {
			return message, err
		}
		if message.outcome == 0 && message.diagnostic != "" {
			return message, fmt.Errorf("successful control outcome contains an error")
		}
	default:
		return message, fmt.Errorf("unknown control message")
	}
	if reader.Len() != 0 {
		return message, fmt.Errorf("control message contains trailing bytes")
	}
	return message, nil
}

func controlErrorText(err error) string {
	if err == nil {
		return ""
	}
	value := strings.ToValidUTF8(strings.ReplaceAll(err.Error(), "\x00", "?"), "?")
	if len(value) > 4096 {
		value = value[:4084]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		value += " [truncated]"
	}
	return value
}
