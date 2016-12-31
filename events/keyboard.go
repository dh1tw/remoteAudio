package events

import (
	"bufio"
	"fmt"
	"os"

	"github.com/cskr/pubsub"
)

const (
	EVENTS = "events"
)

type Event struct {
	SendAudio bool
}

type EventsConf struct {
	EventsPubSub *pubsub.PubSub
}

func CaptureKeyboard(conf EventsConf) {

	ptt := false

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if scanner.Scan() {
			if scanner.Text() == "p" {
				ptt = !ptt
				ev := Event{}
				ev.SendAudio = ptt
				conf.EventsPubSub.Pub(ev, EVENTS)
				fmt.Println("keyboard - ptt:", ptt)
			} else {
				fmt.Println("keyboard input:", scanner.Text())
			}
		}
	}
}
