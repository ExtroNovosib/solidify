package fixture

type Renderer interface{ Render() string }

func Render(r Renderer) string { return r.Render() }
