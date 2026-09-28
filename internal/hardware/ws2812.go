// Package hardware isolates real hardware access (SPI/I2C/GPIO/sysfs) behind
// small interfaces so domain packages can be unit-tested without a Pi.
package hardware

import (
	"fmt"

	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// SPIPort is the WS2812 strip's SPI bus on a Pironman 5 base edition: GPIO10
// (SPI0 MOSI).
const SPIPort = "SPI0.0"

// NumLEDs is the Pironman 5 base edition's fixed onboard WS2812 LED count
// (SunFounder docs: 4 LEDs on GPIO10/SPI0 MOSI). Not a config field in v1 —
// per PLAN.md §4, case-specific hardware constants stay out of the schema
// until a variant needs them configurable.
const NumLEDs = 4

const spiFrequency = 2400 * physic.KiloHertz // ~2.4 MHz — 3 SPI bits per WS2812 bit

// WS2812Strip is the hardware boundary for the addressable RGB LED strip.
type WS2812Strip interface {
	On() error
	Off() error
	SetColor(r, g, b byte)
}

// SPIWS2812 drives a WS2812 strip over SPI at a settable solid color.
type SPIWS2812 struct {
	conn    spi.Conn
	numLEDs int
	r, g, b byte
}

// NewSPIWS2812 opens port and returns a strip driver that lights numLEDs
// pixels at the given color when On.
func NewSPIWS2812(port string, numLEDs int, r, g, b byte) (*SPIWS2812, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("init periph host: %w", err)
	}
	p, err := spireg.Open(port)
	if err != nil {
		return nil, fmt.Errorf("open SPI port %s: %w", port, err)
	}
	conn, err := p.Connect(spiFrequency, spi.Mode0, 8)
	if err != nil {
		return nil, fmt.Errorf("configure SPI connection on %s: %w", port, err)
	}
	return &SPIWS2812{conn: conn, numLEDs: numLEDs, r: r, g: g, b: b}, nil
}

func (s *SPIWS2812) On() error {
	if err := s.conn.Tx(encodeWS2812(s.numLEDs, s.r, s.g, s.b), nil); err != nil {
		return fmt.Errorf("write WS2812 on-state: %w", err)
	}
	return nil
}

func (s *SPIWS2812) Off() error {
	if err := s.conn.Tx(encodeWS2812(s.numLEDs, 0, 0, 0), nil); err != nil {
		return fmt.Errorf("write WS2812 off-state: %w", err)
	}
	return nil
}

// SetColor updates the color a subsequent On() writes to the strip; it does
// not write to the bus itself.
func (s *SPIWS2812) SetColor(r, g, b byte) {
	s.r, s.g, s.b = r, g, b
}

const (
	bitsPerColorBit = 3
	// resetBytes is the trailing all-zero latch gap: 140 bytes * 8 bits /
	// 2.4 MHz ≈ 467us, comfortably above WS2812's minimum reset window.
	//
	// ponytail: fixed conservative constant rather than a per-variant value;
	// revisit if a WS2812 variant needing a different reset window shows up.
	resetBytes = 140
)

// wsBitPattern encodes a WS2812 "0" and "1" data bit as 3 SPI bits each,
// timed by spiFrequency to match the WS2812 protocol's high/low bit widths.
var wsBitPattern = [2]byte{0b100, 0b110}

// encodeWS2812 renders numLEDs solid-color GRB pixels into a WS2812 SPI bitstream.
func encodeWS2812(numLEDs int, r, g, b byte) []byte {
	buf := make([]byte, 0, numLEDs*9+resetBytes)

	var acc uint32
	var accBits int
	writeByte := func(v byte) {
		for bit := 7; bit >= 0; bit-- {
			pattern := wsBitPattern[(v>>uint(bit))&1]
			acc = acc<<bitsPerColorBit | uint32(pattern)
			accBits += bitsPerColorBit
			for accBits >= 8 {
				accBits -= 8
				buf = append(buf, byte(acc>>uint(accBits)))
			}
		}
	}

	for i := 0; i < numLEDs; i++ {
		writeByte(g)
		writeByte(r)
		writeByte(b)
	}

	return append(buf, make([]byte, resetBytes)...)
}
