package bincraft

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Aircraft represents a decoded aircraft from a binCraft record.
type Aircraft struct {
	Hex      string
	Callsign string
	Lat      float64
	Lon      float64
	AltBaro  int
	GS       float64
	Track    float64
	TypeCode string

	HasPosition bool
	HasAltBaro  bool
	HasGS       bool
	HasTrack    bool
	HasCallsign bool
}

// Response represents a decoded binCraft response.
type Response struct {
	Timestamp float64
	Stride    int
	Aircraft  []Aircraft
}

var zstdDecoder *zstd.Decoder

func init() {
	var err error
	zstdDecoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(err)
	}
}

// Decode parses a zstd-compressed binCraft response into a list of aircraft.
func Decode(data []byte) (*Response, error) {
	raw, err := zstdDecoder.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}
	return DecodeRaw(raw)
}

// DecodeRaw parses an uncompressed binCraft binary blob.
func DecodeRaw(raw []byte) (*Response, error) {
	if len(raw) < 52 {
		return nil, fmt.Errorf("binCraft data too short: %d bytes", len(raw))
	}

	// Parse header
	nowLow := binary.LittleEndian.Uint32(raw[0:4])
	nowHigh := binary.LittleEndian.Uint32(raw[4:8])
	timestamp := float64(nowLow)/1000.0 + float64(nowHigh)*4294967.296

	stride := int(binary.LittleEndian.Uint32(raw[8:12]))
	if stride < 112 || stride > 256 {
		return nil, fmt.Errorf("unexpected stride: %d", stride)
	}

	if len(raw) < stride {
		return nil, fmt.Errorf("data shorter than one stride")
	}

	numAircraft := (len(raw) - stride) / stride
	resp := &Response{
		Timestamp: timestamp,
		Stride:    stride,
		Aircraft:  make([]Aircraft, 0, numAircraft),
	}

	for i := 0; i < numAircraft; i++ {
		off := stride + i*stride
		if off+112 > len(raw) {
			break
		}
		rec := raw[off : off+stride]

		ac := decodeAircraft(rec)
		resp.Aircraft = append(resp.Aircraft, ac)
	}

	return resp, nil
}

func decodeAircraft(rec []byte) Aircraft {
	hexRaw := int32(binary.LittleEndian.Uint32(rec[0:4]))
	hexCode := hexRaw & 0xFFFFFF

	// Validity bitmasks
	valid73 := rec[73]
	valid74 := rec[74]

	ac := Aircraft{
		Hex: fmt.Sprintf("%06x", hexCode),
	}

	// Position (byte 73, bit 6)
	if valid73&(1<<6) != 0 {
		ac.Lat = float64(int32(binary.LittleEndian.Uint32(rec[12:16]))) / 1e6
		ac.Lon = float64(int32(binary.LittleEndian.Uint32(rec[8:12]))) / 1e6
		ac.HasPosition = true
	}

	// Baro altitude (byte 73, bit 4)
	if valid73&(1<<4) != 0 {
		ac.AltBaro = int(int16(binary.LittleEndian.Uint16(rec[20:22]))) * 25
		ac.HasAltBaro = true
	}

	// Ground speed (byte 73, bit 7)
	if valid73&(1<<7) != 0 {
		ac.GS = float64(int16(binary.LittleEndian.Uint16(rec[34:36]))) / 10.0
		ac.HasGS = true
	}

	// Track (byte 74, bit 3)
	if valid74&(1<<3) != 0 {
		ac.Track = float64(int16(binary.LittleEndian.Uint16(rec[40:42]))) / 90.0
		ac.HasTrack = true
	}

	// Callsign (byte 73, bit 3)
	if valid73&(1<<3) != 0 {
		ac.Callsign = strings.TrimRight(string(rec[78:86]), "\x00 ")
		ac.HasCallsign = true
	}

	// Type code (bytes 88-91)
	ac.TypeCode = strings.TrimRight(string(rec[88:92]), "\x00 ")

	return ac
}
