package driver

import (
	"fmt"
)

type Service struct {
	current QueueDriver
	drivers map[string]QueueDriver
}

func NewService(defaultDriver QueueDriver, others ...QueueDriver) *Service {
	s := &Service{drivers: make(map[string]QueueDriver)}
	if defaultDriver != nil {
		s.drivers[defaultDriver.Name()] = defaultDriver
		s.current = defaultDriver
	}
	for _, d := range others {
		if d == nil {
			continue
		}
		s.drivers[d.Name()] = d
	}
	return s
}

func (s *Service) UseDriver(name string) error {
	d, ok := s.drivers[name]
	if !ok {
		return fmt.Errorf("invalid queue driver: %s", name)
	}
	s.current = d
	return nil
}

func (s *Service) Driver() QueueDriver { return s.current }
