package hls

import "testing"

// tsPacket pads a PSI section into a full transport packet for the given PID.
func tsPacket(pid int, section []byte) []byte {
	pkt := make([]byte, TSPacketSize)
	for i := range pkt {
		pkt[i] = 0xff
	}
	pkt[0] = tsSyncByte
	pkt[1] = 0x40 | byte(pid>>8&0x1f) // payload_unit_start_indicator + PID high
	pkt[2] = byte(pid & 0xff)
	pkt[3] = 0x10 // payload only, continuity 0
	pkt[4] = 0x00 // pointer_field
	copy(pkt[5:], section)
	return pkt
}

func TestParseTracks(t *testing.T) {
	pat := []byte{
		0x00,       // table_id (PAT)
		0xb0, 0x0d, // section_syntax_indicator + section_length(13)
		0x00, 0x01, // transport_stream_id
		0xc1,       // version/current_next
		0x00, 0x00, // section_number / last_section_number
		0x00, 0x01, // program_number 1
		0xf0, 0x00, // program_map_PID 0x1000
		0x00, 0x00, 0x00, 0x00, // CRC (ignored)
	}

	pmt := []byte{
		0x02,       // table_id (PMT)
		0xb0, 0x22, // section_syntax_indicator + section_length(34)
		0x00, 0x01, // program_number 1
		0xc1,       // version/current_next
		0x00, 0x00, // section_number / last_section_number
		0xe1, 0x00, // PCR_PID 0x0100
		0xf0, 0x00, // program_info_length 0
		// video H.264, PID 0x0100
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
		// audio AAC, PID 0x0101, ISO 639 language "eng"
		0x0f, 0xe1, 0x01, 0xf0, 0x06, 0x0a, 0x04, 'e', 'n', 'g', 0x00,
		// audio AC-3, PID 0x0102
		0x81, 0xe1, 0x02, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, // CRC (ignored)
	}

	stream := append(tsPacket(0x0000, pat), tsPacket(0x1000, pmt)...)
	tracks := ParseTracks(stream)

	if len(tracks) != 3 {
		t.Fatalf("ParseTracks() returned %d tracks, want 3: %+v", len(tracks), tracks)
	}
	want := []Track{
		{PID: 0x0100, Kind: "video", Codec: "H.264", StreamType: 0x1b},
		{PID: 0x0101, Kind: "audio", Codec: "AAC", StreamType: 0x0f, Language: "eng"},
		{PID: 0x0102, Kind: "audio", Codec: "AC-3", StreamType: 0x81},
	}
	for i, w := range want {
		if tracks[i] != w {
			t.Fatalf("track %d = %+v, want %+v", i, tracks[i], w)
		}
	}

	audio := 0
	for _, tr := range tracks {
		if tr.Kind == "audio" {
			audio++
		}
	}
	if audio != 2 {
		t.Fatalf("audio track count = %d, want 2", audio)
	}
}

func TestParseTracksIgnoresNonTS(t *testing.T) {
	if tracks := ParseTracks([]byte("not a transport stream")); tracks != nil {
		t.Fatalf("ParseTracks(non-TS) = %+v, want nil", tracks)
	}
}
