package pbm_test

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/pbm"
)

func checkerboard(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%2 == 0 {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return img
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := checkerboard(16, 10)

	var buf bytes.Buffer
	if err := pbm.Encode(&buf, want); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	got, err := pbm.Decode(&buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Bounds() != want.Bounds() {
		t.Fatalf("Bounds() = %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := 0; y < 10; y++ {
		for x := 0; x < 16; x++ {
			if got.GrayAt(x, y) != want.GrayAt(x, y) {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got.GrayAt(x, y), want.GrayAt(x, y))
			}
		}
	}
}

func TestEncodeFileDecodeFileRoundTrip(t *testing.T) {
	want := checkerboard(9, 7)
	path := filepath.Join(t.TempDir(), "out.pbm")

	if err := pbm.EncodeFile(path, want); err != nil {
		t.Fatalf("EncodeFile: %v", err)
	}

	got, err := pbm.DecodeFile(path)
	if err != nil {
		t.Fatalf("DecodeFile: %v", err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("Bounds() = %v, want %v", got.Bounds(), want.Bounds())
	}
}

func TestDecodeP1ASCII(t *testing.T) {
	src := "P1\n# a comment\n3 2\n0 1 0\n1 0 1\n"
	img, err := pbm.Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 3 || b.Dy() != 2 {
		t.Fatalf("Bounds() = %v, want 3x2", b)
	}
	// bit 1 means black (Y=0) per the PBM spec, bit 0 means white (Y=255).
	want := [][]uint8{{255, 0, 255}, {0, 255, 0}}
	for y, row := range want {
		for x, y8 := range row {
			if got := img.GrayAt(x, y).Y; got != y8 {
				t.Fatalf("pixel (%d,%d) = %d, want %d", x, y, got, y8)
			}
		}
	}
}

func TestDecodeRejectsUnsupportedMagic(t *testing.T) {
	if _, err := pbm.Decode(strings.NewReader("P5\n1 1\n\x00")); err == nil {
		t.Fatalf("expected an error for an unsupported PBM magic")
	}
}

func TestDecodeFilePropagatesMissingFile(t *testing.T) {
	if _, err := pbm.DecodeFile(filepath.Join(t.TempDir(), "missing.pbm")); err == nil {
		t.Fatalf("expected an error for a missing file")
	}
}

func TestEncodeP4HeaderAndPacking(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 3, 1))
	img.SetGray(0, 0, color.Gray{Y: 255})
	img.SetGray(1, 0, color.Gray{Y: 0})
	img.SetGray(2, 0, color.Gray{Y: 255})

	var buf bytes.Buffer
	if err := pbm.Encode(&buf, img); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	const header = "P4\n3 1\n"
	data := buf.Bytes()
	if string(data[:len(header)]) != header {
		t.Fatalf("header = %q, want %q", data[:len(header)], header)
	}
	// 1 row, 3 px wide -> 1 packed byte. bit1 (lit, Y=255) -> 0; bit0 (unlit, Y=0) -> 1.
	// Pixels: lit, unlit, lit -> bits (MSB first): 0 1 0 -> 0x40.
	rest := data[len(header):]
	if len(rest) != 1 || rest[0] != 0x40 {
		t.Fatalf("packed row = %#x, want [0x40]", rest)
	}
}
