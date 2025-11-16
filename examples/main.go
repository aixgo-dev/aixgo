package main

import (
	"github.com/aixgo-dev/aixgo"
	_ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
	if err := aixgo.Run("config/agents.yaml"); err != nil {
		panic(err)
	}
}
