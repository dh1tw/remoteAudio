package chain

import (
	"errors"
	"log"

	"github.com/dh1tw/remoteAudio/audio"
)

type Chain struct {
	Sources       audio.Selector //rx path sources
	Sinks         audio.Router   //rx path sinks
	defaultSource string
	defaultSink   string
}

func NewChain(opts ...Option) (*Chain, error) {

	nc := &Chain{}
	// Setup receiving path
	fromRadioSinks, err := audio.NewDefaultRouter()
	if err != nil {
		log.Fatal(err)
	}
	nc.Sinks = fromRadioSinks

	fromRadioSources, err := audio.NewDefaultSelector()
	if err != nil {
		log.Fatal(err)
	}
	nc.Sources = fromRadioSources

	nc.Sources.SetOnDataCb(nc.DefaultSourceToSinkCb)

	options := Options{}

	for _, option := range opts {
		option(&options)
	}

	if len(options.DefaultSource) == 0 {
		return nil, errors.New("missing default source")
	}

	if len(options.DefaultSink) == 0 {
		return nil, errors.New("missing default sink")
	}

	nc.defaultSink = options.DefaultSink
	nc.defaultSource = options.DefaultSource

	return nc, nil
}

func (nc *Chain) StartTx() error {
	return nc.Sinks.EnableSink(nc.defaultSink, true)
}

func (nc *Chain) StopTx() error {
	return nc.Sinks.EnableSink(nc.defaultSink, false)
}

func (nc *Chain) DefaultSourceToSinkCb(data audio.Msg) {
	err := nc.Sinks.Write(data)
	if err != nil {
		// handle Error -> remove source
		log.Println(err)
	}
	if data.EOF {
		// switch back to default source
		nc.Sinks.Flush()
		if err := nc.Sources.SetSource(nc.defaultSource); err != nil {
			log.Println(err)
		}
		// if len(nc.dvkPlaying) > 0 {
		// 	defer func() { nc.dvkPlaying = "" }()
		// 	if err := nc.ToRadioSources.RemoveSource(nc.dvkPlaying); err != nil {
		// 		log.Println(err)
		// 	}
		// 	if err := nc.FromRadioSources.RemoveSource(nc.dvkPlaying); err != nil {
		// 		log.Println(err)
		// 	}
		// }
	}
}
