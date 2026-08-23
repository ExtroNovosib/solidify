package godhub

func (h *HubRuntime) TouchCfg() { if h.cfg == nil { return } }

func (h *HubRuntime) ExportCfg() int { if h.cfg != nil { return 1 }; return 0 }

func (h *HubRuntime) TouchStore() { if h.store == nil { return } }

func (h *HubRuntime) ExportStore() int { if h.store != nil { return 1 }; return 0 }

func (h *HubRuntime) TouchRouter() { if h.router == nil { return } }

func (h *HubRuntime) ExportRouter() int { if h.router != nil { return 1 }; return 0 }

func (h *HubRuntime) TouchIngress() { if h.ingress == nil { return } }

func (h *HubRuntime) ExportIngress() int { if h.ingress != nil { return 1 }; return 0 }

func (h *HubRuntime) TouchDataplane() { if h.dataplane == nil { return } }

func (h *HubRuntime) ExportDataplane() int { if h.dataplane != nil { return 1 }; return 0 }

func (h *HubRuntime) TouchProxy() { if h.proxy == nil { return } }

func (h *HubRuntime) ExportProxy() int { if h.proxy != nil { return 1 }; return 0 }