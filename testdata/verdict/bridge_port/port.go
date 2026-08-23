package bridgeport

type Manager struct {
	name string
}

func (m *Manager) Start() error {
	return nil
}

type RuntimePort struct {
	mgr *Manager
}

func (p *RuntimePort) Run() error {
	return p.mgr.Start()
}
