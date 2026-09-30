package hardware

// CaseFanRelayOffset is the BCM GPIO line driving the case-fan relay,
// confirmed against pm_auto's gpio_fan default_config (pin 6).
const CaseFanRelayOffset = 6

type Relay interface {
	Set(on bool) error
}
