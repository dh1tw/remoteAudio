package events

import (
	"bufio"
	"os"

	"github.com/cskr/pubsub"
)

const (
	EVENTS = "events"
)

type Event struct {
	EnableLoopback bool
	Echo           bool
}

type EventsConf struct {
	EventsPubSub *pubsub.PubSub
}

func CaptureKeyboard(conf EventsConf) {

	enableLoopback := false

	for {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			if scanner.Text() == "m" {
				enableLoopback = !enableLoopback
				ev := Event{}
				ev.EnableLoopback = enableLoopback
				conf.EventsPubSub.Pub(ev, EVENTS)
			}
		}
	}
}
