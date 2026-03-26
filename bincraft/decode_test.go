package bincraft

import (
	"os"
	"testing"
)

func TestDecodeSample(t *testing.T) {
	data, err := os.ReadFile("/tmp/bincraft_sample.bin")
	if err != nil {
		t.Skip("no sample file at /tmp/bincraft_sample.bin")
	}

	resp, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Timestamp: %f", resp.Timestamp)
	t.Logf("Stride: %d", resp.Stride)
	t.Logf("Aircraft count: %d", len(resp.Aircraft))

	if len(resp.Aircraft) == 0 {
		t.Fatal("expected at least one aircraft")
	}

	// Log first few aircraft with positions
	count := 0
	for _, ac := range resp.Aircraft {
		if !ac.HasPosition {
			continue
		}
		t.Logf("  %s %-8s lat=%.4f lon=%.4f alt=%d gs=%.1f trk=%.1f type=%s",
			ac.Hex, ac.Callsign, ac.Lat, ac.Lon, ac.AltBaro, ac.GS, ac.Track, ac.TypeCode)
		count++
		if count >= 10 {
			break
		}
	}

	// Sanity checks on first aircraft with position
	for _, ac := range resp.Aircraft {
		if !ac.HasPosition {
			continue
		}
		if ac.Lat < -90 || ac.Lat > 90 {
			t.Errorf("latitude out of range: %f", ac.Lat)
		}
		if ac.Lon < -180 || ac.Lon > 180 {
			t.Errorf("longitude out of range: %f", ac.Lon)
		}
		if len(ac.Hex) != 6 {
			t.Errorf("hex should be 6 chars: %q", ac.Hex)
		}
		break
	}
}
