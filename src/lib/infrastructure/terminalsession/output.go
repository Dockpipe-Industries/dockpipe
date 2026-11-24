package terminalsession

// Output is a byte-oriented snapshot, not text: UTF-8/control sequences may span
// reads. Cursor advances only over returned bytes. Truncated means the caller
// fell behind Oldest and must reset its terminal renderer before applying Data.
type Output struct {
	Data      []byte
	Cursor    uint64
	Oldest    uint64
	Truncated bool
	Closed    bool
}

type outputRing struct {
	data [MaxOutput]byte
	end  uint64
}

func (ring *outputRing) append(data []byte) {
	end := ring.end + uint64(len(data))
	if len(data) > MaxOutput {
		data = data[len(data)-MaxOutput:]
	}
	start := end - uint64(len(data))
	position := int(start % MaxOutput)
	first := copy(ring.data[position:], data)
	copy(ring.data[:], data[first:])
	ring.end = end
}

func (ring *outputRing) read(cursor uint64, limit int) (Output, error) {
	if limit < 1 || limit > MaxOutput || cursor > ring.end {
		return Output{}, ErrInvalid
	}
	oldest := uint64(0)
	if ring.end > MaxOutput {
		oldest = ring.end - MaxOutput
	}
	output := Output{Oldest: oldest, Truncated: cursor < oldest}
	if cursor < oldest {
		cursor = oldest
	}
	count := min(uint64(limit), ring.end-cursor)
	output.Data = make([]byte, int(count))
	position := int(cursor % MaxOutput)
	first := copy(output.Data, ring.data[position:min(position+int(count), MaxOutput)])
	copy(output.Data[first:], ring.data[:int(count)-first])
	output.Cursor = cursor + count
	return output, nil
}
