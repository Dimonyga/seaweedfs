package weed_server

import (
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/seaweedfs/seaweedfs/weed/util"
)

const (
	hlsTsVirtualPrefix = "/.hls/"
	hlsTsMetadataKey   = "Seaweed-Hls-Ts-Metadata"
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

func hlsTsEnabled() bool {
	return util.GetViper().GetBool("filer.hls.ts.enabled")
}

// hlsTsMaxSegmentBytes follows the same maxMB semantics as normal filer
// auto-chunking: a positive request maxMB overrides the filer-wide MaxMB,
// otherwise the regular filer MaxMB is inherited.
func (fs *FilerServer) hlsTsMaxSegmentBytes(r *http.Request) int64 {
	parsedMaxMB, _ := strconv.ParseInt(r.URL.Query().Get("maxMB"), 10, 32)
	maxMB := int32(parsedMaxMB)
	if maxMB <= 0 && fs.option.MaxMB > 0 {
		maxMB = int32(fs.option.MaxMB)
	}
	return int64(maxMB) * 1024 * 1024
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

func (fs *FilerServer) maybeHandleHlsTs(w http.ResponseWriter, r *http.Request) bool {
	if !hlsTsEnabled() {
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
