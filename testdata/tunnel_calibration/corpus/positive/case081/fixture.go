package fixture

import "example.com/tunnelcalibration/domain"

func Metadata(f domain.Format) string {
	switch f {
	case domain.JSON:
		return "json"
	case domain.Raw:
		return "raw"
	case domain.Curl:
		return "curl"
	default:
		return ""
	}
}
func Render(f domain.Format) string {
	switch f {
	case domain.JSON:
		return "{}"
	case domain.Raw:
		return "GET /"
	case domain.Curl:
		return "curl"
	default:
		return ""
	}
}
