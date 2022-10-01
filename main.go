package main

import (
	"log"

	"github.com/nstehr/lampy/hue"
)

func main() {
	b, err := hue.DiscoverBridge()
	if err != nil {
		log.Fatal("Could not discover bridge", err)
	}

	err = b.Authenticate("lampy", "v1.0")
	if err != nil {
		log.Fatal("Error authenticating", err)
	}

	light, err := b.GetLightByName("Lampy")
	if err != nil {
		log.Fatal("Error getting light ", err)
	}
	log.Println(light)
}
