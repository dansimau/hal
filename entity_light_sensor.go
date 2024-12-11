package hal

import "strconv"

type LightSensor struct {
	*Entity
}

func NewLightSensor(id string) *LightSensor {
	return &LightSensor{Entity: NewEntity(id)}
}

func (s *LightSensor) Level() int {
	v, err := strconv.Atoi(s.GetState().State)
	if err != nil {
		return 0
	}

	return v
}
