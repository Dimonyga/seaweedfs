package weed_server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	media_hls "github.com/seaweedfs/seaweedfs/weed/media/hls"
)

func TestParseHlsTsRequest(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		method     string
		belongs    bool
		kind       hlsTsRequestKind
		sourcePath string
		sequence   int64
	}{
		{name: "unrelated", path: "/video/movie", method: http.MethodGet, belongs: false},
		{name: "ingest", path: "/hls/video/movie", method: http.MethodPost, belongs: true, kind: hlsTsRequestIngest, sourcePath: "/video/movie"},
		{name: "put ingest", path: "/hls/video/movie", method: http.MethodPut, belongs: true, kind: hlsTsRequestIngest, sourcePath: "/video/movie"},
		{name: "playlist", path: "/hls/video/movie/index.m3u8", method: http.MethodGet, belongs: true, kind: hlsTsRequestPlaylist, sourcePath: "/video/movie"},
		{name: "head playlist", path: "/hls/video/movie/index.m3u8", method: http.MethodHead, belongs: true, kind: hlsTsRequestPlaylist, sourcePath: "/video/movie"},
		{name: "segment", path: "/hls/video/movie/42.ts", method: http.MethodGet, belongs: true, kind: hlsTsRequestSegment, sourcePath: "/video/movie", sequence: 42},
		{name: "negative segment", path: "/hls/video/movie/-1.ts", method: http.MethodGet, belongs: true, kind: hlsTsRequestInvalid},
		{name: "bad segment", path: "/hls/video/movie/nope.ts", method: http.MethodGet, belongs: true, kind: hlsTsRequestInvalid},
		{name: "empty", path: "/hls/", method: http.MethodGet, belongs: true, kind: hlsTsRequestInvalid},
		{name: "unsupported method", path: "/hls/video/movie/index.m3u8", method: http.MethodDelete, belongs: true, kind: hlsTsRequestInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, belongs := parseHlsTsRequest(tt.path, tt.method)
			if belongs != tt.belongs {
				t.Fatalf("belongs = %v, want %v", belongs, tt.belongs)
			}
			if !belongs {
				return
			}
			if got.Kind != tt.kind || got.SourcePath != tt.sourcePath || got.Sequence != tt.sequence {
				t.Fatalf("request = %+v, want kind=%v source=%q sequence=%d", got, tt.kind, tt.sourcePath, tt.sequence)
			}
		})
	}
}

func TestHlsTsJwtSourcePath(t *testing.T) {
	got, ok := hlsTsJwtSourcePath("/hls/buckets/media/movie/7.ts", http.MethodGet)
	if !ok || got != "/buckets/media/movie" {
		t.Fatalf("hlsTsJwtSourcePath() = %q, %v", got, ok)
	}
	if _, ok := hlsTsJwtSourcePath("/hls/buckets/media/movie/nope.ts", http.MethodGet); ok {
		t.Fatal("invalid HLS path unexpectedly produced a JWT source path")
	}
}

func TestHlsTsMaxChunkBytesUsesNormalFilerMaxMB(t *testing.T) {
	fs := &FilerServer{option: &FilerOption{MaxMB: 4}}

	// The limit is aligned down to a whole number of TS packets.
	alignDown := func(n int64) int64 { return n - n%media_hls.TSPacketSize }

	req := httptest.NewRequest(http.MethodPost, "/hls/video/movie", nil)
	if got, want := fs.hlsTsMaxChunkBytes(req), alignDown(4*1024*1024); got != want {
		t.Fatalf("default HLS max chunk bytes = %d, want %d", got, want)
	}

	req = httptest.NewRequest(http.MethodPost, "/hls/video/movie?maxMB=8", nil)
	if got, want := fs.hlsTsMaxChunkBytes(req), alignDown(8*1024*1024); got != want {
		t.Fatalf("overridden HLS max chunk bytes = %d, want %d", got, want)
	}

	for _, value := range []string{"0", "-1", "invalid"} {
		req = httptest.NewRequest(http.MethodPost, "/hls/video/movie?maxMB="+value, nil)
		if got, want := fs.hlsTsMaxChunkBytes(req), alignDown(4*1024*1024); got != want {
			t.Fatalf("maxMB=%q produced %d, want inherited %d", value, got, want)
		}
	}
}
