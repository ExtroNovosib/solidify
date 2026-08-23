package godconsole

import "net/http"

func (m *ConsoleRuntime) TouchCfg() { _ = m.cfg }
func (m *ConsoleRuntime) TouchTemplates() { m.templatesMu.Lock(); _ = m.templates; m.templatesMu.Unlock() }
func (m *ConsoleRuntime) TouchSettings() { _ = m.settings }
func (m *ConsoleRuntime) TouchRoles() { _ = m.roles }
func (m *ConsoleRuntime) TouchQueries() { _ = m.queries }
func (m *ConsoleRuntime) TouchAssets() { _ = m.assets }
func (m *ConsoleRuntime) TouchDevReload() { _ = m.devReload }
func (m *ConsoleRuntime) TouchMinifier() { _ = m.minifier }
func (m *ConsoleRuntime) TouchSpaPolicy() { _ = m.spaPolicy }
func (m *ConsoleRuntime) TouchAuditTrail() { _ = m.auditTrail }
func (m *ConsoleRuntime) TouchCacheEpoch() { _ = m.cacheEpoch }
func (m *ConsoleRuntime) TouchFeatureBits() { _ = m.featureBits }

func (m *ConsoleRuntime) ServeDashboard(w http.ResponseWriter, r *http.Request) { m.TouchCfg(); m.TouchTemplates() }
func (m *ConsoleRuntime) ServeTunnels(w http.ResponseWriter, r *http.Request) { m.TouchQueries(); m.TouchAssets() }
func (m *ConsoleRuntime) ServeDomains(w http.ResponseWriter, r *http.Request) { m.TouchRoles(); m.TouchAuditTrail() }
func (m *ConsoleRuntime) ServeSecurity(w http.ResponseWriter, r *http.Request) { m.TouchFeatureBits(); m.TouchCacheEpoch() }
func (m *ConsoleRuntime) ServeSettings(w http.ResponseWriter, r *http.Request) { m.TouchSettings(); m.TouchDevReload() }
func (m *ConsoleRuntime) ServeUsers(w http.ResponseWriter, r *http.Request) { m.TouchSpaPolicy(); m.TouchMinifier() }
func (m *ConsoleRuntime) ServeAudit(w http.ResponseWriter, r *http.Request) { m.TouchAuditTrail(); m.TouchQueries() }
func (m *ConsoleRuntime) ServeBilling(w http.ResponseWriter, r *http.Request) { m.TouchAssets(); m.TouchCfg() }
func (m *ConsoleRuntime) ServeAdmin(w http.ResponseWriter, r *http.Request) { m.TouchRoles(); m.TouchTemplates() }
func (m *ConsoleRuntime) ServeStatic(w http.ResponseWriter, r *http.Request) { m.TouchAssets(); m.TouchTemplates() }
func (m *ConsoleRuntime) ServeDevProxy(w http.ResponseWriter, r *http.Request) { m.TouchDevReload(); m.TouchSpaPolicy() }
func (m *ConsoleRuntime) HandleHealth(w http.ResponseWriter, r *http.Request) {}
func (m *ConsoleRuntime) GuardAdmin(next http.Handler) http.Handler { return next }
func (m *ConsoleRuntime) GuardSession(next http.Handler) http.Handler { return next }
