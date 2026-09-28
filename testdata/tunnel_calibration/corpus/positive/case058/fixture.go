package fixture

import "example.com/tunnelcalibration/dep"

type Service struct{ DeleteTunnel *dep.Service }

func (c *Service) Handle() error {
	if c.DeleteTunnel == nil {
		return nil
	}
	return c.DeleteTunnel.Handle()
}
