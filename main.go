package main

import (
	"log"

	"github.com/leg100/kubesftp/internal"
)

func main() {
	if err := internal.Run(); err != nil {
		log.Fatal(err)
	}
}
