package hal

// BinarySensor is any sensor with a state of "on" or "off".
type BinarySensor struct {
	*Entity
}

func NewBinarySensor(id string) *BinarySensor {
	return &BinarySensor{Entity: NewEntity(id)}
}

func (s *BinarySensor) IsOff() bool {
	return s.GetState().State == "off"
}

func (s *BinarySensor) IsOn() bool {
	return s.GetState().State == "on"
}
