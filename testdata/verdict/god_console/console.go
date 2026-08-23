package godconsole

import (
	"html/template"
	"net/http"
	"sync"
)

type RuntimeConfig interface {
	Title() string
}

type SettingsSvc struct{}
type RoleSvc interface{ Role(string) (string, bool) }
type QueryRepo interface{}

type ConsoleRuntime struct {
	cfg         RuntimeConfig
	templates   map[string]*template.Template
	templatesMu sync.RWMutex
	settings    SettingsSvc
	roles       RoleSvc
	queries     QueryRepo
	assets      map[string]string
	devReload   bool
	minifier    interface{}
	spaPolicy   interface{}
	auditTrail  map[string]int
	cacheEpoch  int64
	featureBits uint32
}

func NewConsoleRuntime(cfg RuntimeConfig, _ interface{}) *ConsoleRuntime {
	return &ConsoleRuntime{cfg: cfg, templates: make(map[string]*template.Template), settings: SettingsSvc{}, assets: make(map[string]string), auditTrail: make(map[string]int)}
}

func (m *ConsoleRuntime) RenderPage(w http.ResponseWriter, name string) { w.WriteHeader(http.StatusOK) }
func (m *ConsoleRuntime) RenderLogin(w http.ResponseWriter) { w.WriteHeader(http.StatusOK) }
func (m *ConsoleRuntime) HandleSettings(w http.ResponseWriter, r *http.Request) {}
