// Package pbm decodes and encodes the PBM (Portable Bitmap) image format:
// P1 (ASCII) and P4 (binary) on read, P4 on write. Per the format's
// convention, a set bit is black (Y=0) and a clear bit is white (Y=255).
package pbm

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strconv"
)

func Decode(r io.Reader) (*image.Gray, error) {
	br := bufio.NewReader(r)

	magic, err := readToken(br)
	if err != nil {
		return nil, fmt.Errorf("pbm: read magic: %w", err)
	}
	if magic != "P1" && magic != "P4" {
		return nil, fmt.Errorf("pbm: unsupported magic %q", magic)
	}

	width, err := readIntToken(br)
	if err != nil {
		return nil, fmt.Errorf("pbm: read width: %w", err)
	}
	height, err := readIntToken(br)
	if err != nil {
		return nil, fmt.Errorf("pbm: read height: %w", err)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("pbm: invalid dimensions %dx%d", width, height)
	}

	img := image.NewGray(image.Rect(0, 0, width, height))

	if magic == "P1" {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				bit, err := readIntToken(br)
				if err != nil {
					return nil, fmt.Errorf("pbm: read pixel (%d,%d): %w", x, y, err)
				}
				img.SetGray(x, y, bitToGray(bit))
			}
		}
		return img, nil
	}

	rowBytes := (width + 7) / 8
	row := make([]byte, rowBytes)
	for y := 0; y < height; y++ {
		if _, err := io.ReadFull(br, row); err != nil {
			return nil, fmt.Errorf("pbm: read row %d: %w", y, err)
		}
		for x := 0; x < width; x++ {
			bit := (row[x/8] >> (7 - uint(x%8))) & 1
			img.SetGray(x, y, bitToGray(int(bit)))
		}
	}
	return img, nil
}

func DecodeFile(path string) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pbm: open %s: %w", path, err)
	}
	defer f.Close()

	img, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("pbm: decode %s: %w", path, err)
	}
	return img, nil
}

func Encode(w io.Writer, img *image.Gray) error {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if _, err := fmt.Fprintf(w, "P4\n%d %d\n", width, height); err != nil {
		return fmt.Errorf("pbm: write header: %w", err)
	}

	rowBytes := (width + 7) / 8
	row := make([]byte, rowBytes)
	for y := 0; y < height; y++ {
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < width; x++ {
			if img.GrayAt(b.Min.X+x, b.Min.Y+y).Y < 128 {
				row[x/8] |= 1 << (7 - uint(x%8))
			}
		}
		if _, err := w.Write(row); err != nil {
			return fmt.Errorf("pbm: write row %d: %w", y, err)
		}
	}
	return nil
}

func EncodeFile(path string, img *image.Gray) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("pbm: create %s: %w", path, err)
	}
	defer f.Close()

	if err := Encode(f, img); err != nil {
		return err
	}
	return f.Close()
}

func bitToGray(bit int) color.Gray {
	if bit == 1 {
		return color.Gray{Y: 0}
	}
	return color.Gray{Y: 255}
}

// readToken returns the next whitespace-delimited token, skipping leading
// whitespace and "#"-prefixed comments per the PBM header grammar.
func readToken(br *bufio.Reader) (string, error) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '#' {
			if _, err := br.ReadString('\n'); err != nil {
				return "", err
			}
			continue
		}
		if isPBMSpace(b) {
			continue
		}

		buf := []byte{b}
		for {
			b, err := br.ReadByte()
			if err != nil {
				if err == io.EOF {
					return string(buf), nil
				}
				return "", err
			}
			if isPBMSpace(b) {
				return string(buf), nil
			}
			buf = append(buf, b)
		}
	}
}

func readIntToken(br *bufio.Reader) (int, error) {
	tok, err := readToken(br)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(tok)
}

func isPBMSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
