package fixture

import "example.com/tunnelcalibration/dep"

type Manager struct{}

func (*Manager) WireWebhookInbox() any { return dep.NewService() }
