package consumer

import "example.com/externaliface/api"

func Use(value api.Wide) {
	value.A()
}
