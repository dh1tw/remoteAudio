package pbReader

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/golang/protobuf/proto"
)

// PbReader implements the audio.Source interface and is used to read
// audio frames encapsulated in Protocol Buffer messages, typically
// received from the network.
type PbReader struct {
	sync.RWMutex
	options    Options
	enabled    bool
	lastPacket time.Time
}

// NewPbReader is the constructor for a PbReader object.
func NewPbReader(opts ...Option) (*PbReader, error) {

	pbr := &PbReader{
		options: Options{
			DeviceName: "ProtoBufReader",
			Channels:   2,
			Samplerate: 48000,
		},
	}

	for _, option := range opts {
		option(&pbr.options)
	}

	// if no decoder was passed in as a function we create
	// our default opus decoder
	if pbr.options.Decoder == nil {
		decChannels := opus.Channels(pbr.options.Channels)
		decSR := opus.Samplerate(pbr.options.Samplerate)

		dec, err := opus.NewOpusDecoder(decChannels, decSR)
		if err != nil {
			return nil, err
		}
		pbr.options.Decoder = dec
	}

	return pbr, nil
}

// Start processing Protobufs
func (pbr *PbReader) Start() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = true
	return nil
}

// Stop processing Protobufs
func (pbr *PbReader) Stop() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = false
	return nil
}

// Close shuts down the PbReader
func (pbr *PbReader) Close() error {
	return nil
}

// SetCb sets the callback which get's executed once the
// Protobuf has been converted in an audio.Msg.
func (pbr *PbReader) SetCb(cb audio.OnDataCb) {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.options.Callback = cb
}

// Enqueue is the entry point for the PbReader. Incoming Protobufs
// are enqueded with this function.
func (pbr *PbReader) Enqueue(data []byte) error {
	pbr.Lock()
	defer pbr.Unlock()

	if pbr.options.Callback == nil {
		return nil
	}

	if !pbr.enabled {
		return nil
	}

	if pbr.options.Decoder == nil {
		return errors.New("no decoder set")
	}

	if len(data) == 0 {
		log.Println("incoming audio frame empty")
		return nil
	}

	msg := sbAudio.Frame{}
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return err
	}

	if len(msg.Data) == 0 {
		log.Println("protobuf audio frame empty")
		return nil
	}

	if msg.Codec.String() != pbr.options.Decoder.Name() {
		return fmt.Errorf("unknown codec %v", msg.Codec.String())
	}

	buf := make([]float32, msg.FrameLength*2, 5000)

	num, err := pbr.options.Decoder.Decode(msg.Data, buf)
	if err != nil {
		return err
	}

	// pack the data into an audio.Msg which is used for further internal
	// processing
	audioMsg := audio.Msg{
		Channels:   pbr.options.Channels,
		Data:       buf,
		EOF:        false,
		Frames:     num,
		Samplerate: pbr.options.Samplerate, // we want 48kHz for internal processing
		Metadata:   map[string]interface{}{"userID": msg.GetUserId()},
	}

	pbr.options.Callback(audioMsg)

	return nil
}
