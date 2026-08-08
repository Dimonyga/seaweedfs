package hls

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWalkSegmentPayloads(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 6,
		Segments: []Segment{
			{Offset: 0, Size: 3, Duration: 6},
			{Offset: 3, Size: 2, Duration: 6},
		},
	}
	var got [][]byte
	err := WalkSegmentPayloads(bytes.NewBufferString("abcde"), metadata, 16, func(_ int, _ Segment, data []byte) error {
		got = append(got, append([]byte(nil), data...))
		return nil
	})
	if err != nil {
		t.Fatalf("WalkSegmentPayloads() error = %v", err)
	}
	if len(got) != 2 || string(got[0]) != "abc" || string(got[1]) != "de" {
		t.Fatalf("payloads = %q", got)
	}
}

func TestWalkSegmentPayloadsRejectsTruncatedInput(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 6,
		Segments:       []Segment{{Offset: 0, Size: 4, Duration: 6}},
	}
	err := WalkSegmentPayloads(bytes.NewBufferString("abc"), metadata, 16, func(_ int, _ Segment, _ []byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "read segment 0") {
		t.Fatalf("error = %v, want truncated input error", err)
	}
}

func TestWalkSegmentPayloadsRejectsTrailingInput(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 6,
		Segments:       []Segment{{Offset: 0, Size: 3, Duration: 6}},
	}
	err := WalkSegmentPayloads(bytes.NewBufferString("abcd"), metadata, 16, func(_ int, _ Segment, _ []byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "trailing bytes") {
		t.Fatalf("error = %v, want trailing input error", err)
	}
}

func TestWalkSegmentPayloadsPropagatesCallbackError(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 6,
		Segments:       []Segment{{Offset: 0, Size: 3, Duration: 6}},
	}
	sentinel := errors.New("upload failed")
	err := WalkSegmentPayloads(bytes.NewBufferString("abc"), metadata, 16, func(_ int, _ Segment, _ []byte) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want callback error", err)
	}
}

func TestTotalSize(t *testing.T) {
	metadata := &Metadata{Segments: []Segment{{Size: 10}, {Size: 20}, {Size: 30}}}
	if got := TotalSize(metadata); got != 60 {
		t.Fatalf("TotalSize() = %d, want 60", got)
	}
}
