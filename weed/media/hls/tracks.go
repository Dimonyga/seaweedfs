package hls

import "fmt"

// Track describes one elementary stream carried in the MPEG-TS media, derived
// from the Program Association and Program Map tables. It is best-effort
// metadata: a caller that needs authoritative container details attaches its own
// probe output at ingest instead.
type Track struct {
	PID        int    `json:"pid"`
	Kind       string `json:"kind"`  // "video", "audio", or "data"
	Codec      string `json:"codec"` // human-readable codec name
	StreamType int    `json:"stream_type"`
	Language   string `json:"language,omitempty"` // ISO 639 code when present
}

// ParseTracks walks the MPEG-TS packets in data and returns the elementary
// streams announced by the first program's PAT/PMT. It only reads the tables at
// the start of the stream and handles the common single-packet section layout;
// anything it cannot parse is skipped, so the result is advisory and may be
// empty. data is expected to be 188-byte packet aligned (as stored chunks are).
func ParseTracks(data []byte) []Track {
	pmtPIDs := map[int]bool{}
	for off := 0; off+TSPacketSize <= len(data); off += TSPacketSize {
		pid, pusi, payload, ok := tsPacketPayload(data[off : off+TSPacketSize])
		if !ok || pid != 0 || !pusi {
			continue
		}
		if section := psiSection(payload); section != nil {
			for _, p := range parsePAT(section) {
				pmtPIDs[p] = true
			}
		}
		if len(pmtPIDs) > 0 {
			break
		}
	}
	if len(pmtPIDs) == 0 {
		return nil
	}

	var tracks []Track
	seenTrack := map[int]bool{}
	parsedPMT := map[int]bool{}
	for off := 0; off+TSPacketSize <= len(data); off += TSPacketSize {
		pid, pusi, payload, ok := tsPacketPayload(data[off : off+TSPacketSize])
		if !ok || !pusi || !pmtPIDs[pid] || parsedPMT[pid] {
			continue
		}
		section := psiSection(payload)
		if section == nil {
			continue
		}
		for _, tr := range parsePMT(section) {
			if !seenTrack[tr.PID] {
				seenTrack[tr.PID] = true
				tracks = append(tracks, tr)
			}
		}
		parsedPMT[pid] = true
		if len(parsedPMT) == len(pmtPIDs) {
			break
		}
	}
	return tracks
}

// tsPacketPayload extracts the payload bytes of a transport packet, skipping the
// adaptation field when present. ok is false when the packet carries no payload.
func tsPacketPayload(pkt []byte) (pid int, pusi bool, payload []byte, ok bool) {
	if len(pkt) < 4 || pkt[0] != tsSyncByte {
		return 0, false, nil, false
	}
	pid = int(pkt[1]&0x1f)<<8 | int(pkt[2])
	pusi = pkt[1]&0x40 != 0
	offset := 4
	switch (pkt[3] >> 4) & 0x3 {
	case 0x1: // payload only
	case 0x3: // adaptation field followed by payload
		if len(pkt) < 5 {
			return pid, pusi, nil, false
		}
		offset = 5 + int(pkt[4])
	default: // 0x0 or 0x2: no payload
		return pid, pusi, nil, false
	}
	if offset >= len(pkt) {
		return pid, pusi, nil, false
	}
	return pid, pusi, pkt[offset:], true
}

// psiSection returns the PSI section bytes from a payload whose packet had the
// payload_unit_start_indicator set, honouring the leading pointer_field.
func psiSection(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	start := 1 + int(payload[0])
	if start >= len(payload) {
		return nil
	}
	return payload[start:]
}

// parsePAT returns the program_map_PIDs advertised by a Program Association
// Table section, ignoring the network PID (program number 0).
func parsePAT(section []byte) []int {
	if len(section) < 8 || section[0] != 0x00 {
		return nil
	}
	end := 3 + (int(section[1]&0x0f)<<8 | int(section[2]))
	if end > len(section) {
		end = len(section)
	}
	var pids []int
	for i := 8; i+4 <= end-4; i += 4 {
		program := int(section[i])<<8 | int(section[i+1])
		pid := int(section[i+2]&0x1f)<<8 | int(section[i+3])
		if program != 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// parsePMT returns the elementary streams described by a Program Map Table
// section.
func parsePMT(section []byte) []Track {
	if len(section) < 12 || section[0] != 0x02 {
		return nil
	}
	end := 3 + (int(section[1]&0x0f)<<8 | int(section[2]))
	if end > len(section) {
		end = len(section)
	}
	i := 12 + (int(section[10]&0x0f)<<8 | int(section[11])) // skip program_info descriptors
	var tracks []Track
	for i+5 <= end-4 {
		streamType := section[i]
		pid := int(section[i+1]&0x1f)<<8 | int(section[i+2])
		esInfoLen := int(section[i+3]&0x0f)<<8 | int(section[i+4])
		esInfoStart := i + 5
		esInfoEnd := esInfoStart + esInfoLen
		if esInfoEnd > end-4 {
			esInfoEnd = end - 4
		}
		kind, codec := streamKindAndCodec(streamType)
		tracks = append(tracks, Track{
			PID:        pid,
			Kind:       kind,
			Codec:      codec,
			StreamType: int(streamType),
			Language:   parseLanguageDescriptor(section[esInfoStart:esInfoEnd]),
		})
		i = esInfoEnd
	}
	return tracks
}

// parseLanguageDescriptor returns the ISO 639 language code from an ISO 639
// language descriptor (tag 0x0A) if the elementary stream carries one.
func parseLanguageDescriptor(desc []byte) string {
	for i := 0; i+2 <= len(desc); {
		length := int(desc[i+1])
		value := i + 2
		if value+length > len(desc) {
			break
		}
		if desc[i] == 0x0a && length >= 3 {
			return string(desc[value : value+3])
		}
		i = value + length
	}
	return ""
}

// streamKindAndCodec maps an MPEG-TS stream_type to a track kind and a
// human-readable codec name.
func streamKindAndCodec(streamType byte) (kind, codec string) {
	switch streamType {
	case 0x01:
		return "video", "MPEG-1 Video"
	case 0x02:
		return "video", "MPEG-2 Video"
	case 0x10:
		return "video", "MPEG-4 Video"
	case 0x1b:
		return "video", "H.264"
	case 0x24:
		return "video", "H.265"
	case 0x03:
		return "audio", "MP2"
	case 0x04:
		return "audio", "MP2"
	case 0x0f:
		return "audio", "AAC"
	case 0x11:
		return "audio", "AAC-LATM"
	case 0x81:
		return "audio", "AC-3"
	case 0x87:
		return "audio", "E-AC-3"
	default:
		return "data", fmt.Sprintf("stream_type 0x%02X", streamType)
	}
}
