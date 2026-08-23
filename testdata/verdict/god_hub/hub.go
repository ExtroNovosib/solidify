package godhub

// HubRuntime coordinates lifecycle, ingress, proxy, and control paths.
type HubRuntime struct {
	cfg       *HubConfig
	store     *RecordStore
	router    *HostRouter
	ingress   *IngressUnit
	dataplane *DataUnit
	proxy     *ProxyUnit
	control   *ControlUnit
	transport *TransportUnit
	limits    *LimitGate
	audit     *AuditBus
	metrics   *MetricSink
	session   *SessionTable
	signing   *SignProvider
	tokens    *TokenVault
	resumed   map[string]int64
}

type HubConfig struct{}
type RecordStore struct{}
type HostRouter struct{}
type IngressUnit struct{}
type DataUnit struct{}
type ProxyUnit struct{}
type ControlUnit struct{}
type TransportUnit struct{}
type LimitGate struct{}
type AuditBus struct{}
type MetricSink struct{}
type SessionTable struct{}
type SignProvider struct{}
type TokenVault struct{}

func NewHubRuntime(cfg *HubConfig, store *RecordStore, router *HostRouter, ingress *IngressUnit, dataplane *DataUnit, proxy *ProxyUnit) *HubRuntime {
	return &HubRuntime{cfg: cfg, store: store, router: router, ingress: ingress, dataplane: dataplane, proxy: proxy}
}

func NewHubRuntimeWithPolicies(cfg *HubConfig, store *RecordStore, router *HostRouter, ingress *IngressUnit, dataplane *DataUnit, proxy *ProxyUnit, control *ControlUnit, transport *TransportUnit, limits *LimitGate, audit *AuditBus, metrics *MetricSink) *HubRuntime {
	return &HubRuntime{cfg: cfg, store: store, router: router, ingress: ingress, dataplane: dataplane, proxy: proxy, control: control, transport: transport, limits: limits, audit: audit, metrics: metrics}
}
