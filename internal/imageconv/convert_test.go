package imageconv_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/imageconv"
	"github.com/dnitros/pironman/internal/pbm"
)

func solidImage(w, h int, y uint8) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = y
	}
	return img
}

func countLit(img *image.Gray) int {
	n := 0
	for _, v := range img.Pix {
		if v != 0 {
			n++
		}
	}
	return n
}

func TestConvertOutputHasExactTargetSize(t *testing.T) {
	out := imageconv.Convert(solidImage(50, 200, 128), 128, 64)
	if b := out.Bounds(); b.Dx() != 128 || b.Dy() != 64 {
		t.Fatalf("Bounds() = %v, want 128x64", b)
	}
}

func TestConvertSolidBlackProducesNoLitPixels(t *testing.T) {
	out := imageconv.Convert(solidImage(128, 64, 0), 128, 64)
	if n := countLit(out); n != 0 {
		t.Fatalf("lit pixel count = %d, want 0 for a solid black source", n)
	}
}

func TestConvertSolidWhiteProducesMostlyLitPixels(t *testing.T) {
	out := imageconv.Convert(solidImage(128, 64, 255), 128, 64)
	total := 128 * 64
	if n := countLit(out); n < total/2 {
		t.Fatalf("lit pixel count = %d, want more than half of %d for a solid white source", n, total)
	}
}

func TestConvertLetterboxesAndCentersNonMatchingAspectRatio(t *testing.T) {
	out := imageconv.Convert(solidImage(100, 100, 255), 128, 64)
	// scale = min(128/100, 64/100) = 0.64 -> 64x64 centered horizontally, ox=32.
	if got := out.GrayAt(0, 0).Y; got != 0 {
		t.Fatalf("corner pixel (0,0) = %d, want 0 (unlit letterbox area)", got)
	}
}

func TestConvertFlattensTransparentBackgroundOntoWhite(t *testing.T) {
	// Mirrors a typical icon asset: a black glyph on a transparent
	// background, with the transparent pixels' stored color also at (0,0,0)
	// — the case that previously collapsed to a solid black, blank result.
	src := image.NewNRGBA(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	for y := 0; y < hardware.SSD1306Height; y++ {
		for x := 0; x < hardware.SSD1306Width; x++ {
			if x < hardware.SSD1306Width/2 {
				src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			} else {
				src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
			}
		}
	}

	out := imageconv.Convert(src, hardware.SSD1306Width, hardware.SSD1306Height)

	litTransparentHalf, litOpaqueHalf := 0, 0
	for y := 0; y < hardware.SSD1306Height; y++ {
		for x := 0; x < hardware.SSD1306Width/2; x++ {
			if out.GrayAt(x, y).Y != 0 {
				litTransparentHalf++
			}
			if out.GrayAt(x+hardware.SSD1306Width/2, y).Y != 0 {
				litOpaqueHalf++
			}
		}
	}
	half := (hardware.SSD1306Width / 2) * hardware.SSD1306Height
	if litTransparentHalf < half/2 {
		t.Fatalf("lit pixels in the transparent half = %d, want most of %d (flattened onto white, not left black)", litTransparentHalf, half)
	}
	if litOpaqueHalf != 0 {
		t.Fatalf("lit pixels in the opaque-black half = %d, want 0", litOpaqueHalf)
	}
}

func TestPersistImageConvertsPNGAndPersistsPBM(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "photo.png")
	writePNG(t, srcPath, solidImage(64, 64, 255))

	destDir := t.TempDir()
	destPath, err := imageconv.PersistImage(srcPath, destDir, "image-0")
	if err != nil {
		t.Fatalf("PersistImage: %v", err)
	}
	if filepath.Ext(destPath) != ".pbm" {
		t.Fatalf("destPath = %q, want a .pbm file", destPath)
	}
	if filepath.Dir(destPath) != destDir {
		t.Fatalf("destPath dir = %q, want %q", filepath.Dir(destPath), destDir)
	}

	img, err := pbm.DecodeFile(destPath)
	if err != nil {
		t.Fatalf("pbm.DecodeFile: %v", err)
	}
	if b := img.Bounds(); b.Dx() != hardware.SSD1306Width || b.Dy() != hardware.SSD1306Height {
		t.Fatalf("Bounds() = %v, want %dx%d", b, hardware.SSD1306Width, hardware.SSD1306Height)
	}
}

func TestPersistImageAcceptsCorrectlySizedPBM(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "a.pbm")
	if err := pbm.EncodeFile(srcPath, solidImage(hardware.SSD1306Width, hardware.SSD1306Height, 255)); err != nil {
		t.Fatalf("EncodeFile: %v", err)
	}

	destDir := t.TempDir()
	destPath, err := imageconv.PersistImage(srcPath, destDir, "image-0")
	if err != nil {
		t.Fatalf("PersistImage: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected the persisted file to exist: %v", err)
	}
}

func TestPersistImageDistinctDestNamesAvoidCollisionForSameBasename(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	srcA := filepath.Join(dirA, "cat.png")
	srcB := filepath.Join(dirB, "cat.png")
	writePNG(t, srcA, solidImage(32, 32, 255))
	writePNG(t, srcB, solidImage(32, 32, 0))

	destDir := t.TempDir()
	destA, err := imageconv.PersistImage(srcA, destDir, "image-0")
	if err != nil {
		t.Fatalf("PersistImage(A): %v", err)
	}
	destB, err := imageconv.PersistImage(srcB, destDir, "image-1")
	if err != nil {
		t.Fatalf("PersistImage(B): %v", err)
	}
	if destA == destB {
		t.Fatalf("expected distinct destNames to produce distinct paths, both resolved to %q", destA)
	}

	imgA, err := pbm.DecodeFile(destA)
	if err != nil {
		t.Fatalf("pbm.DecodeFile(A): %v", err)
	}
	imgB, err := pbm.DecodeFile(destB)
	if err != nil {
		t.Fatalf("pbm.DecodeFile(B): %v", err)
	}
	if countLit(imgA) == 0 || countLit(imgB) != 0 {
		t.Fatalf("expected A (white source) and B (black source) to persist independently, got lit(A)=%d lit(B)=%d", countLit(imgA), countLit(imgB))
	}
}

func TestPersistImageRejectsWrongSizedPBM(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "wrong.pbm")
	if err := pbm.EncodeFile(srcPath, solidImage(32, 32, 255)); err != nil {
		t.Fatalf("EncodeFile: %v", err)
	}

	if _, err := imageconv.PersistImage(srcPath, t.TempDir(), "image-0"); err == nil {
		t.Fatalf("expected an error for a .pbm that isn't exactly %dx%d", hardware.SSD1306Width, hardware.SSD1306Height)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
}
