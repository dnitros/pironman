package oled

import (
	"image"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
)

func TestFitLineLeavesShortLinesUnchanged(t *testing.T) {
	if got := fitLine(textFace, "CPU 12%", hardware.SSD1306Width); got != "CPU 12%" {
		t.Fatalf("fitLine(%q) = %q, want unchanged", "CPU 12%", got)
	}
}

func TestFitLineTruncatesToFitWidth(t *testing.T) {
	long := "enx0123456789ab 192.168.100.100"
	if font.MeasureString(textFace, long) <= fixed.I(hardware.SSD1306Width) {
		t.Fatalf("test fixture %q must be wider than the display to be meaningful", long)
	}

	got := fitLine(textFace, long, hardware.SSD1306Width)

	if font.MeasureString(textFace, got) > fixed.I(hardware.SSD1306Width) {
		t.Fatalf("fitLine(%q) = %q, still too wide for the display", long, got)
	}
	if got == long {
		t.Fatalf("expected fitLine to actually truncate %q", long)
	}
}

func TestTextRendersOnlyFullyLitOrDarkPixels(t *testing.T) {
	small := renderLines([]string{"CPU 0.5% 35.3°C", "RAM: 0.6/7.9 GB", "DISK: 7.3/14.0", "DISCONNECTED"})
	big := newFrame()
	drawText(big, bigFace, "100°C 2400", 0, 0, pixelOn)
	drawText(big, bigFace, "OFFLINE", 0, 30, pixelOn)

	for _, img := range []*image.Gray{small, big} {
		for i, v := range img.Pix {
			if v != 0 && v != 255 {
				t.Fatalf("pixel %d has gray level %d, want 0 or 255 so the 1-bit threshold loses nothing", i, v)
			}
		}
	}
}
