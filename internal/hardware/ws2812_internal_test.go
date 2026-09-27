package hardware

import "testing"

func TestEncodeWS2812Length(t *testing.T) {
	tests := []struct {
		numLEDs int
		want    int
	}{
		{numLEDs: 1, want: 9 + resetBytes},
		{numLEDs: 4, want: 36 + resetBytes},
	}
	for _, tt := range tests {
		got := len(encodeWS2812(tt.numLEDs, 0, 0, 0))
		if got != tt.want {
			t.Fatalf("encodeWS2812(%d): got length %d, want %d", tt.numLEDs, got, tt.want)
		}
	}
}

func TestEncodeWS2812OffIsZeroBitPattern(t *testing.T) {
	got := encodeWS2812(1, 0, 0, 0)
	want := []byte{0x92, 0x49, 0x24, 0x92, 0x49, 0x24, 0x92, 0x49, 0x24}
	for i, b := range want {
		if got[i] != b {
			t.Fatalf("off pixel byte %d: got 0x%02x, want 0x%02x", i, got[i], b)
		}
	}
}

func TestEncodeWS2812OnIsOneBitPattern(t *testing.T) {
	got := encodeWS2812(1, 0xff, 0xff, 0xff)
	want := []byte{0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6}
	for i, b := range want {
		if got[i] != b {
			t.Fatalf("on pixel byte %d: got 0x%02x, want 0x%02x", i, got[i], b)
		}
	}
}

func TestEncodeWS2812ResetTailIsZero(t *testing.T) {
	got := encodeWS2812(1, 0xff, 0xff, 0xff)
	tail := got[len(got)-resetBytes:]
	for i, b := range tail {
		if b != 0 {
			t.Fatalf("reset tail byte %d: got 0x%02x, want 0x00", i, b)
		}
	}
}
