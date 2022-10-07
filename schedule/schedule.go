package schedule

import (
	"log"
	"net/http"
	"time"

	ics "github.com/arran4/golang-ical"
)

type Schedule struct {
	calUrl    string
	events    []*Event
	lastFetch time.Time
}

type Event struct {
	Start       time.Time
	Summary     string
	Description string
}

func NewSchedule(calUrl string) *Schedule {
	return &Schedule{calUrl: calUrl}
}

// returns the events between now and the end specified time
func (s *Schedule) Upcoming(end time.Duration) ([]*Event, error) {

	events, err := s.GetEvents()
	if err != nil {
		return nil, err
	}

	var matchedEvents []*Event
	for _, event := range events {
		now := TruncateToDay(time.Now())
		if (event.Start.Equal(now)) || event.Start.After(now) && event.Start.Before(now.Add(end)) {
			matchedEvents = append(matchedEvents, event)
		}

	}
	return matchedEvents, nil
}

func (s *Schedule) GetEvents() ([]*Event, error) {
	if time.Now().Before(s.lastFetch.Add(time.Hour * 72)) {
		log.Println("returning cached events")
		return s.events, nil
	}
	log.Println("fetching fresh calendar data")
	resp, err := http.Get(s.calUrl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	cal, err := ics.ParseCalendar(resp.Body)
	if err != nil {
		return nil, err
	}
	events, err := flatten(cal)
	if err != nil {
		return nil, err
	}
	s.events = events
	s.lastFetch = time.Now()
	return events, nil
}

func TruncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func flatten(cal *ics.Calendar) ([]*Event, error) {
	var events []*Event
	for _, event := range cal.Events() {
		e := Event{}
		for _, prop := range event.Properties {
			if prop.IANAToken == "DTSTART" {
				parseTime, err := time.Parse("20060102", prop.Value)
				if err != nil {
					return nil, err
				}

				e.Start = parseTime
				events = append(events, &e)

			}
			if prop.IANAToken == "DESCRIPTION" {
				e.Description = prop.Value
			}
			if prop.IANAToken == "SUMMARY" {
				e.Summary = prop.Value
			}
		}
	}
	return events, nil
}
