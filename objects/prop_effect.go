package objects

import (
	"strconv"
	"time"
)

type Effect struct {
	startTime time.Time
	length    time.Duration

	lastTrigger time.Time
	interval    int
	triggers    int
	effect      func(triggers int)
	effectOff   func()
	magnitude   int
}

func (s *Effect) AlterTime(duration float64) {
	s.length = time.Duration(duration * float64(time.Second))
}

func (s *Effect) ExtendDuration(duration float64) {
	calc := s.length - (s.length - (time.Now().Sub(s.startTime)))
	s.length = time.Duration(duration)*time.Second - time.Duration(calc.Seconds())
}

func NewEffect(length string, interval int, magnitude int, effect func(triggers int), effectOff func()) *Effect {
	lengthTime, _ := strconv.Atoi(length)
	parseLength := time.Duration(lengthTime) * time.Second
	return &Effect{time.Now(),
		parseLength,
		time.Now(),
		interval,
		0,
		effect,
		effectOff,
		magnitude}
}

func (s *Effect) RunEffect() {
	s.effect(s.triggers)
	s.triggers += 1
	s.lastTrigger = time.Now()
}

func (s *Effect) Reset(t time.Duration) {
	s.startTime = time.Now()
	s.length = t
}

func (s *Effect) TimeRemaining() float64 {
	calc := s.length - (time.Now().Sub(s.startTime))
	return calc.Seconds()
}

func (s *Effect) LastTriggerInterval() int {
	lTrigger := time.Now().Sub(s.lastTrigger)
	calc := s.interval - int(lTrigger.Seconds())
	return calc
}

// Function to return only the modifiable properties
func (s *Effect) ReturnEffectProps() map[string]interface{} {
	serialList := map[string]interface{}{
		"timeRemaining": s.TimeRemaining(),
		"magnitude":     s.magnitude,
	}
	return serialList
}
