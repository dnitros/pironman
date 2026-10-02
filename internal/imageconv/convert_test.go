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
	out := imageconv.Convert(solidImage(50, 200, 128), 128, 64, false)
	if b := out.Bounds(); b.Dx() != 128 || b.Dy() != 64 {
		t.Fatalf("Bounds() = %v, want 128x64", b)
	}
}

func TestConvertSolidBlackProducesNoLitPixels(t *testing.T) {
	out := imageconv.Convert(solidImage(128, 64, 0), 128, 64, false)
	if n := countLit(out); n != 0 {
		t.Fatalf("lit pixel count = %d, want 0 for a solid black source", n)
	}
}

func TestConvertSolidWhiteProducesMostlyLitPixels(t *testing.T) {
	out := imageconv.Convert(solidImage(128, 64, 255), 128, 64, false)
	total := 128 * 64
	if n := countLit(out); n < total/2 {
		t.Fatalf("lit pixel count = %d, want more than half of %d for a solid white source", n, total)
	}
}

func TestConvertSolidWhiteProducesAllLitPixels(t *testing.T) {
	// A boundary-scaled Bayer threshold (0, 17, ..., 255) leaves a periodic
	// grid of pixels unlit even for pure white (255 > 255 is false at the top
	// cell) — a dead-dot-grid defect, not real dithering. A correctly
	// centered threshold must light every pixel of a flat, maximally bright
	// source.
	out := imageconv.Convert(solidImage(128, 64, 255), 128, 64, false)
	total := 128 * 64
	if n := countLit(out); n != total {
		t.Fatalf("lit pixel count = %d, want all %d lit for a solid white source", n, total)
	}
}

func TestConvertNearBlackSourceProducesNoLitPixels(t *testing.T) {
	// A boundary-scaled Bayer threshold (0, 17, ...) lights up any pixel with
	// Y>=1 at the threshold=0 cell — so faint anti-aliasing noise in an
	// otherwise-solid dark background produces a periodic grid of stray lit
	// dots. A correctly centered threshold must not light a near-black
	// (but not pure-black) flat source at all.
	out := imageconv.Convert(solidImage(128, 64, 1), 128, 64, false)
	if n := countLit(out); n != 0 {
		t.Fatalf("lit pixel count = %d, want 0 for a near-black (Y=1) source", n)
	}
}

func TestConvertLetterboxesAndCentersNonMatchingAspectRatio(t *testing.T) {
	out := imageconv.Convert(solidImage(100, 100, 255), 128, 64, false)
	// scale = min(128/100, 64/100) = 0.64 -> 64x64 centered horizontally, ox=32.
	if got := out.GrayAt(0, 0).Y; got != 0 {
		t.Fatalf("corner pixel (0,0) = %d, want 0 (unlit letterbox area)", got)
	}
}

func TestConvertInvertLeavesLetterboxMarginUnlit(t *testing.T) {
	// A square source letterboxed into a wide canvas must keep the margin
	// dark even when inverted — otherwise invert would grow a lit border
	// around an otherwise-dark result instead of just flipping the logo.
	out := imageconv.Convert(solidImage(100, 100, 0), 128, 64, true)
	if got := out.GrayAt(0, 0).Y; got != 0 {
		t.Fatalf("letterbox corner pixel (0,0) = %d, want 0 (unlit) regardless of invert", got)
	}
	// Within the scaled region, inverting a solid-black source must light it up.
	if got := out.GrayAt(64, 32).Y; got != 255 {
		t.Fatalf("center pixel (64,32) = %d, want 255 (lit) after inverting a solid-black source", got)
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

	out := imageconv.Convert(src, hardware.SSD1306Width, hardware.SSD1306Height, false)

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
	destPath, err := imageconv.PersistImage(srcPath, destDir, "image-0", false)
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
	destPath, err := imageconv.PersistImage(srcPath, destDir, "image-0", false)
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
	destA, err := imageconv.PersistImage(srcA, destDir, "image-0", false)
	if err != nil {
		t.Fatalf("PersistImage(A): %v", err)
	}
	destB, err := imageconv.PersistImage(srcB, destDir, "image-1", false)
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

	if _, err := imageconv.PersistImage(srcPath, t.TempDir(), "image-0", false); err == nil {
		t.Fatalf("expected an error for a .pbm that isn't exactly %dx%d", hardware.SSD1306Width, hardware.SSD1306Height)
	}
}

func TestInvertFlipsEveryPixel(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 4, 1))
	img.SetGray(0, 0, color.Gray{Y: 0})
	img.SetGray(1, 0, color.Gray{Y: 255})
	img.SetGray(2, 0, color.Gray{Y: 0})
	img.SetGray(3, 0, color.Gray{Y: 255})

	out := imageconv.Invert(img)

	want := []uint8{255, 0, 255, 0}
	for x, w := range want {
		if got := out.GrayAt(x, 0).Y; got != w {
			t.Fatalf("pixel %d = %d, want %d", x, got, w)
		}
	}
}

func TestPersistImageWithInvertFlipsPersistedResult(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "photo.png")
	writePNG(t, srcPath, solidImage(hardware.SSD1306Width, hardware.SSD1306Height, 255))

	destDir := t.TempDir()
	plainPath, err := imageconv.PersistImage(srcPath, destDir, "plain", false)
	if err != nil {
		t.Fatalf("PersistImage(invert=false): %v", err)
	}
	invertedPath, err := imageconv.PersistImage(srcPath, destDir, "inverted", true)
	if err != nil {
		t.Fatalf("PersistImage(invert=true): %v", err)
	}

	plain, err := pbm.DecodeFile(plainPath)
	if err != nil {
		t.Fatalf("pbm.DecodeFile(plain): %v", err)
	}
	inverted, err := pbm.DecodeFile(invertedPath)
	if err != nil {
		t.Fatalf("pbm.DecodeFile(inverted): %v", err)
	}

	for i := range plain.Pix {
		if plain.Pix[i] == inverted.Pix[i] {
			t.Fatalf("pixel %d: plain=%d inverted=%d, want every pixel flipped", i, plain.Pix[i], inverted.Pix[i])
		}
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
