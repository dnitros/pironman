// Package pbm decodes and encodes the PBM (Portable Bitmap) image format.
//
// Ported byte-for-byte from github.com/l-you/pironman5-go's
// internal/pbm/pbm.go (GPLv2), for a direct A/B comparison against this
// project's own conversion approach. No project-internal imports, so no
// adaptation was needed beyond the package declaration.
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

func DecodeFile(path string) (*image.Gray, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pbm: %w", err)
	}
	defer file.Close()
	img, err := Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode pbm: %w", err)
	}
	return img, nil
}

func Decode(r io.Reader) (*image.Gray, error) {
	dec := decoder{r: bufio.NewReader(r)}
	magic, err := dec.token()
	if err != nil {
		return nil, err
	}
	width, err := dec.intToken("width")
	if err != nil {
		return nil, err
	}
	height, err := dec.intToken("height")
	if err != nil {
		return nil, err
	}
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("invalid pbm size %dx%d", width, height)
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	switch magic {
	case "P1":
		if err := decodeP1(&dec, img); err != nil {
			return nil, err
		}
	case "P4":
		if err := decodeP4(&dec, img); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported pbm magic %q", magic)
	}
	return img, nil
}

func EncodeFile(path string, img *image.Gray) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create pbm: %w", err)
	}
	defer file.Close()
	if err := Encode(file, img); err != nil {
		return fmt.Errorf("encode pbm: %w", err)
	}
	return nil
}

func Encode(w io.Writer, img *image.Gray) error {
	bounds := img.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return fmt.Errorf("invalid image bounds %v", bounds)
	}
	if _, err := fmt.Fprintf(w, "P4\n%d %d\n", bounds.Dx(), bounds.Dy()); err != nil {
		return err
	}
	rowBytes := (bounds.Dx() + 7) / 8
	row := make([]byte, rowBytes)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		clear(row)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.GrayAt(x, y).Y > 127 {
				continue
			}
			offset := x - bounds.Min.X
			row[offset/8] |= 1 << uint(7-offset%8)
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func decodeP1(dec *decoder, img *image.Gray) error {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			token, err := dec.token()
			if err != nil {
				return err
			}
			switch token {
			case "0":
				img.SetGray(x, y, color.Gray{Y: 255})
			case "1":
			default:
				return fmt.Errorf("invalid P1 pixel %q", token)
			}
		}
	}
	return nil
}

func decodeP4(dec *decoder, img *image.Gray) error {
	if err := dec.skipIgnored(); err != nil {
		return err
	}
	rowBytes := (img.Bounds().Dx() + 7) / 8
	row := make([]byte, rowBytes)
	for y := 0; y < img.Bounds().Dy(); y++ {
		if _, err := io.ReadFull(dec.r, row); err != nil {
			return fmt.Errorf("read P4 row %d: %w", y, err)
		}
		for x := 0; x < img.Bounds().Dx(); x++ {
			if row[x/8]&(1<<uint(7-x%8)) != 0 {
				continue
			}
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	return nil
}

type decoder struct {
	r *bufio.Reader
}

func (d *decoder) intToken(name string) (int, error) {
	token, err := d.token()
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("parse pbm %s %q: %w", name, token, err)
	}
	return value, nil
}

func (d *decoder) token() (string, error) {
	if err := d.skipIgnored(); err != nil {
		return "", err
	}
	buf := make([]byte, 0, 8)
	for {
		b, err := d.r.ReadByte()
		if err != nil {
			if err == io.EOF && len(buf) > 0 {
				return string(buf), nil
			}
			return "", err
		}
		if isSpace(b) {
			return string(buf), nil
		}
		if b == '#' {
			if err := d.skipComment(); err != nil {
				return "", err
			}
			return string(buf), nil
		}
		buf = append(buf, b)
	}
}

func (d *decoder) skipIgnored() error {
	for {
		b, err := d.r.ReadByte()
		if err != nil {
			return err
		}
		switch {
		case isSpace(b):
			continue
		case b == '#':
			if err := d.skipComment(); err != nil {
				return err
			}
		default:
			if err := d.r.UnreadByte(); err != nil {
				return err
			}
			return nil
		}
	}
}

func (d *decoder) skipComment() error {
	for {
		b, err := d.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if b == '\n' || b == '\r' {
			return nil
		}
	}
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}
