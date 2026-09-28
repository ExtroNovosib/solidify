package fixture

import (
	"example.com/tunnelcalibration/domain"
	"net/http"
)

type Store interface {
	Create(*domain.Aggregate)
	Save(*domain.Aggregate)
}
type Command interface{ Handle() }
type Deps struct {
	Store   Store
	Command Command
}
type Handler struct{ deps Deps }

func (h *Handler) handlePublishVersion(w http.ResponseWriter, r *http.Request) {
	a := domain.New()
	h.deps.Store.Create(a)
	h.deps.Store.Save(a)
}
