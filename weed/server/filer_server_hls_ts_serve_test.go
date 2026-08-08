package weed_server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	media_hls "github.com/seaweedfs/seaweedfs/weed/media/hls"
)

func TestHlsTsSegmentForSequence(t *testing.T) {
	metadata := &media_hls.Metadata{
		MediaSequence: 7,
		Segments: []media_hls.Segment{
			{Offset: 0, Size: 188, Duration: 4},
			{Offset: 188, Size: 376, Duration: 4},
		},
	}
	if _, ok := hlsTsSegmentForSequence(metadata, 6); ok {
		t.Fatal("sequence before media sequence unexpectedly resolved")
	}
	if got, ok := hlsTsSegmentForSequence(metadata, 7); !ok || got.Offset != 0 || got.Size != 188 {
		t.Fatalf("first segment = %+v, %v", got, ok)
	}
	if got, ok := hlsTsSegmentForSequence(metadata, 8); !ok || got.Offset != 188 || got.Size != 376 {
		t.Fatalf("second segment = %+v, %v", got, ok)
	}
	if _, ok := hlsTsSegmentForSequence(metadata, 9); ok {
		t.Fatal("sequence after last segment unexpectedly resolved")
	}
}

func TestApplyHlsTsPassthroughHeaders(t *testing.T) {
	entry := &filer.Entry{Extended: map[string][]byte{
		"Cache-Control":        []byte("public, max-age=60"),
		"Expires":              []byte("Wed, 21 Oct 2030 07:28:00 GMT"),
		hlsTsMetadataKey:       []byte("must-not-be-copied-by-this-helper"),
		"Content-Disposition": []byte("attachment"),
	}}
	recorder := httptest.NewRecorder()
	applyHlsTsPassthroughHeaders(recorder, entry)
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get(hlsTsMetadataKey); got != "" {
		t.Fatalf("internal HLS metadata leaked as response header: %q", got)
	}
}

func TestBuildHlsTsMediaInfo(t *testing.T) {
	metadata := &media_hls.Metadata{
		Version:        media_hls.MetadataVersion,
		TargetDuration: 6,
		Segments: []media_hls.Segment{
			{Offset: 0, Size: 100, Duration: 6},
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
	if got.SegmentCount != 2 || got.Duration != 11.5 || got.Size != 220 {
		t.Fatalf("media info = %+v", got)
	}
	if len(got.Tracks) != 2 || got.Tracks[1].Language != "eng" {
		t.Fatalf("tracks = %+v", got.Tracks)
	}
}
