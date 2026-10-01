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

func TestSPIWS2812SetColorUpdatesStoredColor(t *testing.T) {
	s := &SPIWS2812{numLEDs: 1}
	s.SetColor(0x11, 0x22, 0x33)
	if s.r != 0x11 || s.g != 0x22 || s.b != 0x33 {
		t.Fatalf("SetColor: got r=%#x g=%#x b=%#x, want r=0x11 g=0x22 b=0x33", s.r, s.g, s.b)
	}
}

func TestEncodeFrameMatchesEncodeWS2812ForUniformPixels(t *testing.T) {
	pixels := []Color{{R: 0x11, G: 0x22, B: 0x33}, {R: 0x11, G: 0x22, B: 0x33}}
	got := encodeFrame(pixels)
	want := encodeWS2812(2, 0x11, 0x22, 0x33)
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got 0x%02x, want 0x%02x", i, got[i], want[i])
		}
	}
}

func TestEncodeFramePerPixelColors(t *testing.T) {
	got := encodeFrame([]Color{{R: 0, G: 0, B: 0}, {R: 0xff, G: 0xff, B: 0xff}})
	wantPixel0 := []byte{0x92, 0x49, 0x24, 0x92, 0x49, 0x24, 0x92, 0x49, 0x24}
	wantPixel1 := []byte{0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6}
	for i, b := range wantPixel0 {
		if got[i] != b {
			t.Fatalf("pixel 0 byte %d: got 0x%02x, want 0x%02x", i, got[i], b)
		}
	}
	for i, b := range wantPixel1 {
		if got[9+i] != b {
			t.Fatalf("pixel 1 byte %d: got 0x%02x, want 0x%02x", i, got[9+i], b)
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
