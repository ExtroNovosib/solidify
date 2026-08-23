package edgeproxy

import (
	"net/http"
	"time"
)

type TunnelView struct{ ID string }

type RelayEdge struct{}

type statusHandler func(e *RelayEdge, t *TunnelView, id string, start time.Time, w http.ResponseWriter, r *http.Request, timeout time.Duration, force bool, code int) bool

func (e *RelayEdge) callRelayStatus(h statusHandler, t *TunnelView, id string, start time.Time, w http.ResponseWriter, r *http.Request, timeout time.Duration, force bool, code int) bool {
	if e == nil || h == nil {
		return false
	}
	return h(e, t, id, start, w, r, timeout, force, code)
}

func (e *RelayEdge) HandleIngress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
