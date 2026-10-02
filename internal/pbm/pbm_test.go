// Ported byte-for-byte from github.com/l-you/pironman5-go's
// internal/pbm/pbm_test.go (GPLv2), as part of the A/B conversion-code swap.
// Note the white-box `package pbm` (not `pbm_test`) — that's their
// convention, kept here for fidelity even though it differs from this
// project's own black-box-by-default style.
package pbm

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			if (x+y)%2 == 0 {
				src.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, src.Pix) {
		t.Fatalf("decoded pixels = %v, want %v", got.Pix, src.Pix)
	}
}

func TestDecodeP1WithComments(t *testing.T) {
	data := strings.NewReader("P1\n# test\n4 2\n0 1 0 1\n1 0 1 0\n")
	img, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	if img.GrayAt(0, 0).Y != 255 || img.GrayAt(1, 0).Y != 0 {
		t.Fatalf("unexpected first row pixels")
	}
}

func TestDecodeRejectsUnsupportedMagic(t *testing.T) {
	if _, err := Decode(strings.NewReader("P2\n1 1\n0\n")); err == nil {
		t.Fatal("expected unsupported magic error")
	}
}
