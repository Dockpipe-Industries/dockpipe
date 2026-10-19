package archivecodec

import (
	"context"
	"errors"
	"io"
)

var ErrDecodedLimit = errors.New("archive decoded byte limit exceeded")

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.reader.Read(buffer)
	if cancelErr := reader.ctx.Err(); cancelErr != nil {
		return count, cancelErr
	}
	return count, err
}

type boundedReader struct {
	ctx       context.Context
	reader    io.ReadCloser
	remaining int64
	failure   error
}

func (reader *boundedReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if reader.failure != nil {
		return 0, reader.failure
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	if reader.remaining == 0 {
		var probe [1]byte
		count, err := reader.reader.Read(probe[:])
		if count != 0 {
			err = ErrDecodedLimit
		}
		reader.failure = err
		return 0, err
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	count, err := reader.reader.Read(buffer)
	reader.remaining -= int64(count)
	if cancelErr := reader.ctx.Err(); cancelErr != nil {
		err = cancelErr
	}
	reader.failure = err
	return count, err
}

func (reader *boundedReader) Close() error {
	return reader.reader.Close()
}
