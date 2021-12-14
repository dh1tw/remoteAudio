package chain

import (
	"errors"
	"log"

	"github.com/dh1tw/remoteAudio/audio"
)

// Chain holds a complete chain of audio elements from the Source,
// through processing nodes ending in a sink. In a typically VoIP
// architecture one would have one receiving (rx) and transmitting
// (tx) chain.
type Chain struct {
	Sources       audio.Selector //selector can hold one or more sources
	Sinks         audio.Router   //router can hold one or more sinks
	Nodes         []audio.Node
	defaultSource string
	defaultSink   string
}

// NewChain is the constructor method for an audio chain.
func NewChain(opts ...Option) (*Chain, error) {

	nc := &Chain{}

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

	// overwrite default sink / source with values provided by the
	// options variable
	nc.defaultSink = options.DefaultSink
	nc.defaultSource = options.DefaultSource
	nc.Nodes = options.Nodes

	nodesCount := len(nc.Nodes)

	// Wire up the chain, connect the source->nodes->sink with each other
	// if no nodes are available, we connect the source directly with the sink
	if nodesCount == 0 {
		nc.Sources.SetOnDataCb(nc.defaultSourceToSinkCb)
		return nc, nil
	} else { // connect the source with the first node
		nc.Sources.SetOnDataCb(func(msg audio.Msg) {
			nc.Nodes[0].Write(msg)
		})
	}

	// connect the remaining nodes with each other
	for i, node := range nc.Nodes {
		if i == 0 {
			continue // first node is already connected
		}
		previousNode := nc.Nodes[i-1]
		previousNode.SetCb(func(msg audio.Msg) {
			node.Write(msg)
		})
	}

	// connect the last node with the sink
	nc.Nodes[nodesCount-1].SetCb(nc.defaultSourceToSinkCb)

	return nc, nil
}

// Enable will enable or disable the chain. This is done be enabling
// or disableing the selected default sink in the chain.
func (nc *Chain) Enable(state bool) error {
	return nc.Sinks.EnableSink(nc.defaultSink, state)
}

// Enabled returns the state if the chain / it's default
// sink is enabled.
func (nc *Chain) Enabled() (bool, error) {
	_, enabled, err := nc.Sinks.Sink(nc.defaultSink)
	return enabled, err
}

// defaultSourceToSinkCb connects a source with a sink
func (nc *Chain) defaultSourceToSinkCb(data audio.Msg) {
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
