package hardware

// CaseFanRelayLine is the kernel-assigned name of the GPIO line driving the
// case-fan relay (BCM GPIO6, confirmed against pm_auto's gpio_fan
// default_config). Which /dev/gpiochipN this line ends up on varies across
// Pi 5 kernels/OS images (other chips — RP1 I2C/SPI/SDIO lines, HDMI DDC,
// etc. — enumerate ahead of it in an order that isn't fixed), so it must be
// resolved by name, not by a guessed chip number.
const CaseFanRelayLine = "GPIO6"

type Relay interface {
	Set(on bool) error
}
