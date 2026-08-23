package mixedconcerns

import (
	"github.com/ExtroNovosib/solidify/testdata/verdict/httpfake"
	"github.com/ExtroNovosib/solidify/testdata/verdict/sqlfake"
)

type Gateway struct {
	db sqlfake.DB
}

func (g *Gateway) ServeHTTP() httpfake.Response {
	return httpfake.WriteOK()
}

func (g *Gateway) HandleGet() httpfake.Response {
	return httpfake.WriteOK()
}

func (g *Gateway) HandlePost() httpfake.Response {
	return httpfake.WriteCreated()
}

func (g *Gateway) QueryUser(id string) string {
	return sqlfake.QueryRow(g.db, id)
}

func (g *Gateway) InsertUser(name string) error {
	return sqlfake.ExecInsert(g.db, name)
}

func (g *Gateway) DeleteUser(id string) error {
	return sqlfake.ExecDelete(g.db, id)
}
