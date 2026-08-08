package weed_server

import (
	"net/http"
	"path"
	"strconv"
	"strings"

	media_hls "github.com/seaweedfs/seaweedfs/weed/media/hls"
)

const (
	hlsTsVirtualPrefix = "/hls/"
	// Use the existing x-seaweedfs-* internal metadata namespace so normal filer
	// GETs never turn the potentially large segment index into an HTTP header.
	hlsTsMetadataKey = "x-seaweedfs-hls-ts-metadata"

	// hlsTsSegmentPrefetch bounds how many chunks a multi-chunk segment read
	// fetches concurrently. It matches the value the normal filer read handler
	// uses and is capped by the segment's own chunk count.
	hlsTsSegmentPrefetch = 4

	// hlsTsTrackScanBytes bounds how much of the media start is inspected for
	// MPEG-TS PAT/PMT tables while ingesting media-info metadata.
	hlsTsTrackScanBytes = 256 * 1024
)

type hlsTsRequestKind int

const (
	hlsTsRequestInvalid hlsTsRequestKind = iota
	hlsTsRequestIngest
	hlsTsRequestPlaylist
	hlsTsRequestSegment
)

type hlsTsRequest struct {
	SourcePath string
	Kind       hlsTsRequestKind
	Sequence   int64
}

func (fs *FilerServer) hlsTsEnabled() bool {
	return fs.option != nil && fs.option.HlsTsEnabled
}

func (fs *FilerServer) hlsTsMaxChunkBytes(r *http.Request) int64 {
	parsedMaxMB, _ := strconv.ParseInt(r.URL.Query().Get("maxMB"), 10, 32)
	maxMB := int32(parsedMaxMB)
	if maxMB <= 0 && fs.option.MaxMB > 0 {
		maxMB = int32(fs.option.MaxMB)
	}
	limit := int64(maxMB) * 1024 * 1024
	return limit - limit%media_hls.TSPacketSize
}

func parseHlsTsRequest(requestPath string, method string) (hlsTsRequest, bool) {
	if !strings.HasPrefix(requestPath, hlsTsVirtualPrefix) {
		return hlsTsRequest{}, false
	}
	relative := strings.TrimPrefix(requestPath, hlsTsVirtualPrefix)
	if relative == "" {
		return hlsTsRequest{}, true
	}
	if method == http.MethodPost || method == http.MethodPut {
		source := "/" + strings.Trim(relative, "/")
		if source == "/" {
			return hlsTsRequest{}, true
		}
		return hlsTsRequest{SourcePath: path.Clean(source), Kind: hlsTsRequestIngest}, true
	}
	if method != http.MethodGet && method != http.MethodHead {
		return hlsTsRequest{}, true
	}
	if strings.HasSuffix(relative, "/index.m3u8") {
		source := strings.TrimSuffix(relative, "/index.m3u8")
		if source == "" {
			return hlsTsRequest{}, true
		}
		return hlsTsRequest{SourcePath: path.Clean("/" + source), Kind: hlsTsRequestPlaylist}, true
	}
	dir, file := path.Split(relative)
	if dir == "" || !strings.HasSuffix(file, ".ts") {
		return hlsTsRequest{}, true
	}
	sequence, err := strconv.ParseInt(strings.TrimSuffix(file, ".ts"), 10, 64)
	if err != nil || sequence < 0 {
		return hlsTsRequest{}, true
	}
	source := strings.TrimSuffix(dir, "/")
	if source == "" {
		return hlsTsRequest{}, true
	}
	return hlsTsRequest{SourcePath: path.Clean("/" + source), Kind: hlsTsRequestSegment, Sequence: sequence}, true
}

func hlsTsJwtSourcePath(requestPath, method string) (string, bool) {
	parsed, belongs := parseHlsTsRequest(requestPath, method)
	if !belongs || parsed.Kind == hlsTsRequestInvalid {
		return "", false
	}
	return parsed.SourcePath, true
}

func (fs *FilerServer) shouldBypassHlsTsReadJwt(r *http.Request) bool {
	if !fs.hlsTsEnabled() || fs.hlsTsReadJwtRequired.Load() {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	_, belongs := parseHlsTsRequest(r.URL.Path, r.Method)
	return belongs
}

func (fs *FilerServer) maybeHandleHlsTs(w http.ResponseWriter, r *http.Request) bool {
	if !fs.hlsTsEnabled() {
		return false
	}
	parsed, belongs := parseHlsTsRequest(r.URL.Path, r.Method)
	if !belongs {
		return false
	}
	if parsed.Kind == hlsTsRequestInvalid {
		http.Error(w, "invalid HLS TS request", http.StatusBadRequest)
		return true
	}
	switch parsed.Kind {
	case hlsTsRequestIngest:
		fs.hlsTsIngestHandler(w, r, parsed.SourcePath)
	case hlsTsRequestPlaylist:
		fs.hlsTsPlaylistHandler(w, r, parsed.SourcePath)
	case hlsTsRequestSegment:
		fs.hlsTsSegmentHandler(w, r, parsed.SourcePath, parsed.Sequence)
	default:
		http.Error(w, "invalid HLS TS request", http.StatusBadRequest)
	}
	return true
}
