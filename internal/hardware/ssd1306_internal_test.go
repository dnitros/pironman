package hardware

import (
	"image"
	"image/color"
	"testing"
)

func TestPackSSD1306FrameLength(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	got := packSSD1306Frame(img)
	want := SSD1306Width * SSD1306Height / 8
	if len(got) != want {
		t.Fatalf("len(packSSD1306Frame(...)) = %d, want %d", len(got), want)
	}
}

func TestPackSSD1306FrameAllBlackIsAllZero(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	got := packSSD1306Frame(img)
	for i, b := range got {
		if b != 0 {
			t.Fatalf("byte %d = 0x%02x, want 0x00 for an all-black image", i, b)
		}
	}
}

func TestPackSSD1306FrameSetsBitForLitPixel(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	// Pixel (x=3, y=9) sits in page 1 (y/8), bit 1 (y%8).
	img.SetGray(3, 9, color.Gray{Y: 255})

	got := packSSD1306Frame(img)
	const page = 1
	idx := page*SSD1306Width + 3
	if got[idx] != 0b0000_0010 {
		t.Fatalf("frame[%d] = 0x%02x, want 0x02", idx, got[idx])
	}
	for i, b := range got {
		if i != idx && b != 0 {
			t.Fatalf("byte %d = 0x%02x, want 0x00 (only pixel (3,9) is lit)", i, b)
		}
	}
}

func TestPackSSD1306FrameTreatsMidGrayAsUnlit(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	img.SetGray(0, 0, color.Gray{Y: 100})

	got := packSSD1306Frame(img)
	if got[0] != 0 {
		t.Fatalf("frame[0] = 0x%02x, want 0x00 for a sub-threshold gray pixel", got[0])
	}
}
