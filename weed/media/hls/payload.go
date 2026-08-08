package hls

import (
	"fmt"
	"io"
)

// WalkSegmentPayloads reads exactly one payload for each segment in metadata.
// The callback receives a fresh byte slice owned by the caller. The function
// rejects truncated input and trailing bytes, which prevents committing an
// entry whose logical file differs from the playlist used to segment it.
func WalkSegmentPayloads(reader io.Reader, metadata *Metadata, maxSegmentSize int64, fn func(index int, segment Segment, data []byte) error) error {
	if err := Validate(metadata, -1, maxSegmentSize); err != nil {
		return err
	}
	for i, segment := range metadata.Segments {
		if segment.Size > int64(int(^uint(0)>>1)) {
			return fmt.Errorf("segment %d is too large for this platform: %d", i, segment.Size)
		}
		data := make([]byte, int(segment.Size))
		if _, err := io.ReadFull(reader, data); err != nil {
			return fmt.Errorf("read segment %d (%d bytes): %w", i, segment.Size, err)
		}
		if err := fn(i, segment, data); err != nil {
			return fmt.Errorf("process segment %d: %w", i, err)
		}
	}

	var extra [1]byte
	n, err := reader.Read(extra[:])
	if n != 0 {
		return fmt.Errorf("media payload has trailing bytes after the last segment")
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("check media payload end: %w", err)
	}
	if err == nil {
		return fmt.Errorf("media payload did not reach EOF after the last segment")
	}
	return nil
}

func TotalSize(metadata *Metadata) int64 {
	var total int64
	if metadata == nil {
		return 0
	}
	for _, segment := range metadata.Segments {
		total += segment.Size
	}
	return total
}
