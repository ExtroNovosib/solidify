package godhub

func (h *HubRuntime) SyncCfg() error { _ = h.cfg; return nil }

func (h *HubRuntime) ResetCfg() { h.cfg = nil }

func (h *HubRuntime) SyncStore() error { _ = h.store; return nil }

func (h *HubRuntime) ResetStore() { h.store = nil }

func (h *HubRuntime) SyncRouter() error { _ = h.router; return nil }

func (h *HubRuntime) ResetRouter() { h.router = nil }

func (h *HubRuntime) SyncIngress() error { _ = h.ingress; return nil }

func (h *HubRuntime) ResetIngress() { h.ingress = nil }

func (h *HubRuntime) SyncDataplane() error { _ = h.dataplane; return nil }

func (h *HubRuntime) ResetDataplane() { h.dataplane = nil }

func (h *HubRuntime) SyncProxy() error { _ = h.proxy; return nil }

func (h *HubRuntime) ResetProxy() { h.proxy = nil }

func (h *HubRuntime) SyncControl() error { _ = h.control; return nil }

func (h *HubRuntime) ResetControl() { h.control = nil }

func (h *HubRuntime) SyncTransport() error { _ = h.transport; return nil }

func (h *HubRuntime) ResetTransport() { h.transport = nil }

func (h *HubRuntime) SyncLimits() error { _ = h.limits; return nil }

func (h *HubRuntime) ResetLimits() { h.limits = nil }

func (h *HubRuntime) SyncAudit() error { _ = h.audit; return nil }

func (h *HubRuntime) ResetAudit() { h.audit = nil }

func (h *HubRuntime) SyncMetrics() error { _ = h.metrics; return nil }

func (h *HubRuntime) ResetMetrics() { h.metrics = nil }

func (h *HubRuntime) SyncSession() error { _ = h.session; return nil }

func (h *HubRuntime) ResetSession() { h.session = nil }