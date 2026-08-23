package wiringroot

import "example.com/dipverdict/adapters/postgres"

func Wire() {
	_ = postgres.NewClient()
}
