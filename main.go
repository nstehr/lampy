package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucasb-eyer/go-colorful"
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

	b.ToggleLight(light.ID, true)
	b.AdjustBrightness(light.ID, 100)
	c, err := colorful.Hex("#A7226E")
	if err != nil {
		log.Fatal(err)
	}
	b.SetColor(light.ID, c)

	//colors := colorful.FastHappyPalette(5)

	//partyMode(b, light, []colorful.Color{c})

	// Clean exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	<-sig
	// Exit by user
	log.Println("Ctrl-c detected, shutting down")

	log.Println("Goodbye.")
}

func partyMode(b *hue.Bridge, light *hue.Light, pallete []colorful.Color) {
	brightness := 100.0
	decrease := true
	idx := 0
	go func() {
		for range time.Tick(100 * time.Millisecond) {
			if decrease {
				brightness = brightness - 1.0
			}
			if !decrease {
				brightness = brightness + 1.0
			}
			if brightness <= 0.0 || brightness >= 100.0 {
				decrease = !decrease

			}
			if brightness == 0 {
				idx = (idx + 1) % len(pallete)
				c := pallete[idx]
				b.SetColor(light.ID, c)
			}
			log.Println(brightness)
			b.AdjustBrightness(light.ID, brightness)
		}
	}()
}
