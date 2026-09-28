package fixture

import "example.com/tunnelcalibration/dep"

type Service struct{ CreateTunnel *dep.Service }

func (c *Service) Handle() error {
	if c.CreateTunnel == nil {
		return nil
	}
	return c.CreateTunnel.Handle()
}
