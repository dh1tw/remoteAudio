package webserver

import "testing"
import "github.com/cskr/pubsub"

func TestRace(t *testing.T) {

	evPS := pubsub.New(10)

	s := WebServerSettings{
		Events: evPS,
	}

	go Webserver(s)

	for {
		select {}
	}

}
