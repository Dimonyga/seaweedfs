package weed_server

import (
	"encoding/json"
	"testing"

	media_hls "github.com/seaweedfs/seaweedfs/weed/media/hls"
)

func TestBuildHlsTsMediaInfo(t *testing.T) {
	metadata := &media_hls.Metadata{
		Version:        media_hls.MetadataVersion,
		TargetDuration: 6,
		MediaSequence:  0,
		Segments: []media_hls.Segment{
			{Offset: 0, Size: 100, Duration: 6.0},
			{Offset: 100, Size: 120, Duration: 5.5},
		},
		Tracks: []media_hls.Track{
			{PID: 0x100, Kind: "video", Codec: "H.264", StreamType: 0x1b},
			{PID: 0x101, Kind: "audio", Codec: "AAC", StreamType: 0x0f, Language: "eng"},
		},
	}

	body, err := buildHlsTsMediaInfo(metadata, 220)
	if err != nil {
		t.Fatalf("buildHlsTsMediaInfo() error = %v", err)
	}

	var got hlsTsMediaInfo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal media info: %v", err)
	}
	if got.SegmentCount != 2 {
		t.Fatalf("segment_count = %d, want 2", got.SegmentCount)
	}
	if got.Duration != 11.5 {
		t.Fatalf("duration = %v, want 11.5", got.Duration)
	}
	if got.Size != 220 {
		t.Fatalf("size = %d, want 220", got.Size)
	}
	if len(got.Tracks) != 2 || got.Tracks[1].Language != "eng" {
		t.Fatalf("tracks = %+v, want 2 with audio language eng", got.Tracks)
	}

	// The response must not carry any externally supplied metadata field.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw media info: %v", err)
	}
	if _, ok := raw["mediainfo"]; ok {
		t.Fatal("media info unexpectedly carried a passthrough mediainfo field")
	}
}
