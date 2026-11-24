package remote

import (
	"errors"

	contract "dockpipe/src/lib/domain/remote"
)

type terminalBuffer struct {
	data   [contract.MaxTerminalOutput]byte
	oldest uint64
	end    uint64
}

func (buffer *terminalBuffer) append(start uint64, data []byte, truncated bool) error {
	end := start + uint64(len(data))
	if end < start {
		return errors.New("terminal cursor overflow")
	}
	if end <= buffer.end {
		return nil
	} // exact transport replay: already received
	if start < buffer.end {
		data = data[buffer.end-start:]
		start = buffer.end
	}
	if start != buffer.end {
		if !truncated || start < buffer.end {
			return errors.New("terminal output cursor gap")
		}
		buffer.oldest = start
	}
	position := int(start % contract.MaxTerminalOutput)
	first := copy(buffer.data[position:], data)
	copy(buffer.data[:], data[first:])
	buffer.end = end
	if end > contract.MaxTerminalOutput && buffer.oldest < end-contract.MaxTerminalOutput {
		buffer.oldest = end - contract.MaxTerminalOutput
	}
	return nil
}

func (buffer *terminalBuffer) read(cursor uint64) (contract.TerminalView, error) {
	if cursor > buffer.end {
		return contract.TerminalView{}, errors.New("terminal cursor is ahead of output")
	}
	view := contract.TerminalView{Oldest: buffer.oldest, End: buffer.end, Truncated: cursor < buffer.oldest}
	if cursor < buffer.oldest {
		cursor = buffer.oldest
	}
	count := int(min(uint64(contract.MaxTerminalChunk), buffer.end-cursor))
	view.Data = make([]byte, count)
	position := int(cursor % contract.MaxTerminalOutput)
	first := copy(view.Data, buffer.data[position:min(position+count, contract.MaxTerminalOutput)])
	copy(view.Data[first:], buffer.data[:count-first])
	view.Cursor = cursor + uint64(count)
	return view, nil
}
