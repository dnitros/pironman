package hardware

import (
	"image"
	"image/color"
	"testing"
)

func TestPackSSD1306FrameLength(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	got := packSSD1306Frame(img, 0)
	want := SSD1306Width * SSD1306Height / 8
	if len(got) != want {
		t.Fatalf("len(packSSD1306Frame(...)) = %d, want %d", len(got), want)
	}
}

func TestPackSSD1306FrameAllBlackIsAllZero(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	got := packSSD1306Frame(img, 0)
	for i, b := range got {
		if b != 0 {
			t.Fatalf("byte %d = 0x%02x, want 0x00 for an all-black image", i, b)
		}
	}
}

func TestPackSSD1306FrameSetsBitForLitPixel(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))

	img.SetGray(3, 9, color.Gray{Y: 255})

	got := packSSD1306Frame(img, 0)
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

	got := packSSD1306Frame(img, 0)
	if got[0] != 0 {
		t.Fatalf("frame[0] = 0x%02x, want 0x00 for a sub-threshold gray pixel", got[0])
	}
}

func TestPackSSD1306FrameRotates180(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, SSD1306Width, SSD1306Height))
	img.SetGray(3, 9, color.Gray{Y: 255})

	got := packSSD1306Frame(img, 180)

	const wantX, wantY = SSD1306Width - 1 - 3, SSD1306Height - 1 - 9
	wantPage := wantY / 8
	wantIdx := wantPage*SSD1306Width + wantX
	if got[wantIdx]&(1<<uint(wantY%8)) == 0 {
		t.Fatalf("frame[%d] = 0x%02x, want bit %d set for the rotated pixel", wantIdx, got[wantIdx], wantY%8)
	}

	const origPage = 1
	origIdx := origPage*SSD1306Width + 3
	if got[origIdx] != 0 {
		t.Fatalf("frame[%d] = 0x%02x, want 0x00: the unrotated position must stay unlit", origIdx, got[origIdx])
	}
}

func TestValidateRotationRejectsUnsupportedDegrees(t *testing.T) {
	for _, degrees := range []int{0, 180} {
		if err := ValidateRotation(degrees); err != nil {
			t.Fatalf("ValidateRotation(%d) = %v, want nil", degrees, err)
		}
	}
	if err := ValidateRotation(90); err == nil {
		t.Fatalf("ValidateRotation(90) = nil, want an error")
	}
}

func TestSetRotationRecordsValidValueAndRejectsInvalid(t *testing.T) {
	d := &I2CSSD1306{}
	if err := d.SetRotation(180); err != nil {
		t.Fatalf("SetRotation(180): %v", err)
	}
	if d.rotation != 180 {
		t.Fatalf("d.rotation = %d, want 180", d.rotation)
	}

	if err := d.SetRotation(90); err == nil {
		t.Fatalf("SetRotation(90) = nil, want an error")
	}
	if d.rotation != 180 {
		t.Fatalf("d.rotation = %d, want unchanged 180 after a rejected value", d.rotation)
	}
}
