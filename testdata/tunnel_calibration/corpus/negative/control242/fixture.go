package fixture

import "net/http"

func Transform(h http.Header) int { h.Set("X-Mode", "valid"); return http.StatusOK }
