package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/nstehr/lampy/hue"
	"github.com/nstehr/lampy/schedule"
)

var cal = flag.String("cal", "", "ical url for events")

const (
	cardboard = "cardboard"
	garbage   = "garbage"
	week      = time.Hour * 24 * 7
)

func main() {

	cal := os.Getenv("LAMPY_SCHEDULE")
	if cal == "" {
		log.Fatal("no calendar specified")
	}

	b, err := hue.DiscoverBridge()
	if err != nil {
		log.Println("Could not discover bridge", err)
		staticBridge := os.Getenv("HUE_BRIDGE")
		if staticBridge == "" {
			log.Fatal("Could not discover bridge and no fallback provided")
		}
		staticIp := net.ParseIP(staticBridge)
		if staticIp == nil {
			log.Fatalf("Error parsing bridge ip: %s", staticIp)
		}
		b = hue.CreateBridge(staticIp)
	}

	err = b.Authenticate("lampy", "v1.0")
	if err != nil {
		log.Fatal("Error authenticating", err)
	}

	light, err := b.GetLightByName("Lampy")
	if err != nil {
		log.Fatal("Error getting light ", err)
	}

	green, err := colorful.Hex("#04db50")
	if err != nil {
		log.Fatal(err)
	}

	blue, err := colorful.Hex("#0761f2")
	if err != nil {
		log.Fatal(err)
	}

	colourMap := map[string]colorful.Color{garbage: blue, cardboard: green}
	sched := schedule.NewSchedule(cal)
	// get all the events within the week. Since we are using our
	// garbage schedule we can just pull out the first, since there should be
	// just one
	events, err := sched.Upcoming(week)
	if err != nil {
		log.Fatal(err)
	}

	// set the lamp on startup
	b.AdjustBrightness(light.ID, 0)
	if len(events) > 0 {
		setLamp(events[0], b, light, colourMap)
	} else {
		log.Println("No events found")
	}

	// main glue logic here
	go func() {

		for range time.Tick(4 * time.Hour) {
			log.Println("fetching schedule and setting colour")
			events, err := sched.Upcoming(week)
			if err != nil {
				log.Println("Error getting upcoming events:", err)
				continue
			}

			if len(events) <= 0 {
				log.Println("No events found")
				continue
			}
			setLamp(events[0], b, light, colourMap)
		}
	}()

	// Clean exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	<-sig
	// Exit by user
	log.Println("Ctrl-c detected, shutting down")

	log.Println("Goodbye.")
}

func setLamp(event *schedule.Event, b *hue.Bridge, light *hue.Light, colourMap map[string]colorful.Color) {
	b.ToggleLight(light.ID, true)

	now := schedule.TruncateToDay(time.Now())
	diff := event.Start.Sub(now)
	log.Printf("We are %v away\n", diff)
	// if the day is within 24 hours, we'll set it to full brightness, unless
	// it's past noon
	if diff.Seconds() == 0 || diff.Hours() == 24 {
		if diff.Seconds() == 0 && time.Now().Hour() > 12 {
			b.AdjustBrightness(light.ID, 10)
		} else {
			b.AdjustBrightness(light.ID, 100)
		}

	} else if diff.Hours() == 48 {
		// bump the brightness a tad as we get closer
		b.AdjustBrightness(light.ID, 20)
	} else {
		// will flip it off otherwise
		b.AdjustBrightness(light.ID, 0)
	}
	key := garbage

	if strings.Contains(strings.ToLower(event.Summary), "black bin") {
		key = cardboard
	}
	log.Printf("It's %s day", key)
	if c, ok := colourMap[key]; ok {
		b.SetColor(light.ID, c)
	} else {
		log.Println("No matching event summary in colour map")
	}

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
