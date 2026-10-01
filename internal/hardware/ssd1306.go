package hardware

import (
	"fmt"
	"image"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"
)

const I2CPort = "/dev/i2c-1"

const ssd1306Address = 0x3C

const (
	SSD1306Width  = 128
	SSD1306Height = 64
	ssd1306Pages  = SSD1306Height / 8
)

const grayLitThreshold = 128

type SSD1306Display interface {
	Draw(img *image.Gray) error
}

// ssd1306InitCommands is the SSD1306's documented power-on init sequence.
var ssd1306InitCommands = []byte{
	0xAE,
	0xD5, 0x80,
	0xA8, 0x3F,
	0xD3, 0x00,
	0x40,
	0x8D, 0x14,
	0x20, 0x00,
	0xA1,
	0xC8,
	0xDA, 0x12,
	0x81, 0xCF,
	0xD9, 0xF1,
	0xDB, 0x40,
	0xA4,
	0xA6,
	0xAF,
}

var ssd1306AddressRange = []byte{
	0x21, 0x00, SSD1306Width - 1,
	0x22, 0x00, ssd1306Pages - 1,
}

type I2CSSD1306 struct {
	dev *i2c.Dev
}

func NewI2CSSD1306(port string) (*I2CSSD1306, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("init periph host: %w", err)
	}
	bus, err := i2creg.Open(port)
	if err != nil {
		return nil, fmt.Errorf("open I2C port %s: %w", port, err)
	}

	d := &I2CSSD1306{dev: &i2c.Dev{Bus: bus, Addr: ssd1306Address}}
	if err := d.writeCommands(ssd1306InitCommands); err != nil {
		return nil, fmt.Errorf("initialize SSD1306: %w", err)
	}
	return d, nil
}

func (d *I2CSSD1306) writeCommands(cmds []byte) error {
	buf := make([]byte, 0, len(cmds)+1)
	buf = append(buf, 0x00)
	buf = append(buf, cmds...)
	if err := d.dev.Tx(buf, nil); err != nil {
		return fmt.Errorf("write command: %w", err)
	}
	return nil
}

func (d *I2CSSD1306) Draw(img *image.Gray) error {
	b := img.Bounds()
	if b.Dx() != SSD1306Width || b.Dy() != SSD1306Height {
		return fmt.Errorf("draw: image must be %dx%d, got %dx%d", SSD1306Width, SSD1306Height, b.Dx(), b.Dy())
	}

	if err := d.writeCommands(ssd1306AddressRange); err != nil {
		return err
	}

	frame := packSSD1306Frame(img)
	buf := make([]byte, 0, len(frame)+1)
	buf = append(buf, 0x40)
	buf = append(buf, frame...)
	if err := d.dev.Tx(buf, nil); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

func packSSD1306Frame(img *image.Gray) []byte {
	frame := make([]byte, SSD1306Width*ssd1306Pages)
	b := img.Bounds()
	for y := 0; y < SSD1306Height; y++ {
		for x := 0; x < SSD1306Width; x++ {
			if img.GrayAt(b.Min.X+x, b.Min.Y+y).Y < grayLitThreshold {
				continue
			}
			page := y / 8
			bit := uint(y % 8)
			frame[page*SSD1306Width+x] |= 1 << bit
		}
	}
	return frame
}
