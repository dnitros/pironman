package hardware

const CaseFanRelayLine = "GPIO6"

type Relay interface {
	Set(on bool) error
}
