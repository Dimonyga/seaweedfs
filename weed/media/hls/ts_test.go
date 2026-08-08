package hls

import (
	"strings"
	"testing"
)

func TestParseSingleFilePlaylistExplicitOffsets(t *testing.T) {
	playlist := `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-TARGETDURATION:7
#EXT-X-MEDIA-SEQUENCE:3
#EXTINF:6.006,
#EXT-X-BYTERANGE:1000@0
video.ts
#EXTINF:5.994,
#EXT-X-BYTERANGE:900@1000
video.ts
#EXT-X-ENDLIST
`
	metadata, err := ParseSingleFilePlaylist([]byte(playlist))
	if err != nil {
		t.Fatalf("ParseSingleFilePlaylist() error = %v", err)
	}
	if metadata.TargetDuration != 7 || metadata.MediaSequence != 3 {
		t.Fatalf("unexpected playlist metadata: %+v", metadata)
	}
	if len(metadata.Segments) != 2 {
		t.Fatalf("segment count = %d, want 2", len(metadata.Segments))
	}
	if got := metadata.Segments[1]; got.Offset != 1000 || got.Size != 900 || got.Duration != 5.994 {
		t.Fatalf("segment[1] = %+v", got)
	}
}

func TestParseSingleFilePlaylistImplicitOffsets(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:4.0,
#EXT-X-BYTERANGE:1880@0
video.ts
#EXTINF:4.0,
#EXT-X-BYTERANGE:3760
video.ts
#EXT-X-ENDLIST
`
	metadata, err := ParseSingleFilePlaylist([]byte(playlist))
	if err != nil {
		t.Fatalf("ParseSingleFilePlaylist() error = %v", err)
	}
	if metadata.TargetDuration != 4 {
		t.Fatalf("target duration = %d, want 4", metadata.TargetDuration)
	}
	if got := metadata.Segments[1].Offset; got != 1880 {
		t.Fatalf("implicit offset = %d, want 1880", got)
	}
}

func TestParseSingleFilePlaylistRejectsMultipleMediaURIs(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:4,
#EXT-X-BYTERANGE:100@0
one.ts
#EXTINF:4,
#EXT-X-BYTERANGE:100@100
two.ts
#EXT-X-ENDLIST
`
	_, err := ParseSingleFilePlaylist([]byte(playlist))
	if err == nil || !strings.Contains(err.Error(), "not single-file HLS") {
		t.Fatalf("error = %v, want single-file rejection", err)
	}
}

func TestParseSingleFilePlaylistRejectsGap(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:4,
#EXT-X-BYTERANGE:100@0
video.ts
#EXTINF:4,
#EXT-X-BYTERANGE:100@101
video.ts
#EXT-X-ENDLIST
`
	_, err := ParseSingleFilePlaylist([]byte(playlist))
	if err == nil || !strings.Contains(err.Error(), "non-contiguous") {
		t.Fatalf("error = %v, want non-contiguous rejection", err)
	}
}

func TestParseSingleFilePlaylistRequiresEndList(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:4,
#EXT-X-BYTERANGE:100@0
video.ts
`
	_, err := ParseSingleFilePlaylist([]byte(playlist))
	if err == nil || !strings.Contains(err.Error(), "EXT-X-ENDLIST") {
		t.Fatalf("error = %v, want VOD rejection", err)
	}
}

func TestParseSingleFilePlaylistRequiresByteRange(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:4,
segment.ts
#EXT-X-ENDLIST
`
	_, err := ParseSingleFilePlaylist([]byte(playlist))
	if err == nil || !strings.Contains(err.Error(), "EXT-X-BYTERANGE") {
		t.Fatalf("error = %v, want byte-range rejection", err)
	}
}

func TestTargetDurationUsesRFC8216Rounding(t *testing.T) {
	playlist := `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.006,
#EXT-X-BYTERANGE:100@0
video.ts
#EXT-X-ENDLIST
`
	if _, err := ParseSingleFilePlaylist([]byte(playlist)); err != nil {
		t.Fatalf("6.006 second segment is valid with target duration 6 after nearest-integer rounding: %v", err)
	}
}

func TestValidate(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 6,
		Segments: []Segment{
			{Offset: 0, Size: 100, Duration: 6},
			{Offset: 100, Size: 200, Duration: 6},
		},
	}
	if err := Validate(metadata, 300, 256); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := Validate(metadata, 301, 256); err == nil {
		t.Fatal("Validate() accepted mismatched file size")
	}
	if err := Validate(metadata, 300, 150); err == nil {
		t.Fatal("Validate() accepted oversized segment")
	}
}

func TestRenderMediaPlaylistUsesPlainSegmentURIs(t *testing.T) {
	metadata := &Metadata{
		Version:        MetadataVersion,
		TargetDuration: 7,
		MediaSequence:  3,
		Segments: []Segment{
			{Offset: 0, Size: 100, Duration: 6.006},
			{Offset: 100, Size: 200, Duration: 5.994},
		},
	}
	got := string(RenderMediaPlaylist(metadata))
	want := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:7
#EXT-X-MEDIA-SEQUENCE:3
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:6.006,
3.ts
#EXTINF:5.994,
4.ts
#EXT-X-ENDLIST
`
	if got != want {
		t.Fatalf("playlist mismatch\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
