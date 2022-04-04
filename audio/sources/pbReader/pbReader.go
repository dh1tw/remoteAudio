package pbReader

import (
	"fmt"
	"log"
	"sync"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/golang/protobuf/proto"
)

// PbReader implements the audio.Source interface and is used to read
// audio frames encapsulated in Protocol Buffer messages, typically
// received from the network.
type PbReader struct {
	sync.RWMutex
	enabled            bool
	name               string
	decoders           map[string]audiocodec.Decoder
	decoder            audiocodec.Decoder
	callback           audio.OnDataCb
	lastUser           string
	emptyUserIDWarning sync.Once
}

// NewPbReader is the constructor for a PbReader object.
func NewPbReader() (*PbReader, error) {

	pbr := &PbReader{
		name:     "ProtoBufReader",
		decoders: make(map[string]audiocodec.Decoder),
		lastUser: "",
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
	pbr.callback = cb
}

// Enqueue is the entry point for the PbReader. Incoming Protobufs
// are enqueded with this function.
func (pbr *PbReader) Enqueue(data []byte) error {
	pbr.Lock()
	defer pbr.Unlock()

	if !pbr.enabled {
		return nil
	}

	if pbr.callback == nil {
		return nil
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

	channels := 0
	switch msg.GetChannels() {
	case sbAudio.Channels_mono:
		channels = 1
	case sbAudio.Channels_stereo:
		channels = 2
	}

	if len(msg.Data) == 0 {
		log.Println("incoming protobuf audio frame empty")
		return nil
	}

	if len(msg.GetUserId()) == 0 {
		msg.UserId = "unknown-client"
		pbr.emptyUserIDWarning.Do(func() {
			log.Println("incoming audio frames from unknown user; consider setting the username on the client")
			log.Println("simultaneous audio frames from multiple anonymous users will result in distorted audio")
		})
	}

	codecName := msg.GetCodec().String()

	switch codecName {
	case "opus":
	// case "pcm":
	default:
		return fmt.Errorf("unknown codec %v", msg.Codec.String())
	}

	buf := make([]float32, int(msg.GetFrameLength())*channels)

	txUser := msg.GetUserId()

	// we can not use the same opus decoder when packets of multiple
	// users arrive at the same time. This ends up in a very distorted
	// audio. Therefore we create a new decoder on demand for each txUser
	if pbr.lastUser != txUser {
		codec, ok := pbr.decoders[txUser]
		if !ok {
			switch codecName {
			case "opus":
				newCodec, err := newOpusDecoder(channels)
				if err != nil {
					return (err)
				}
				pbr.decoders[txUser] = newCodec
				pbr.decoder = newCodec
			case "pcm":
				// in case of PCM we might have to resample the audio
				// to match the internally prefered 48Khz
			}
		} else {
			pbr.decoder = codec // codec already exists for txUser
		}
		pbr.lastUser = txUser
	}

	if pbr.decoder == nil {
		return fmt.Errorf("no decoder set for audio frames from user: '%s'", txUser)
	}

	num, err := pbr.decoder.Decode(msg.Data, buf)
	if err != nil {
		// in case the txUser has switched from stereo to mono
		// the samples won't fit into buf anymore. Therefore we
		// simple ignore the sample and delete the decoder for that user
		delete(pbr.decoders, txUser)
		pbr.lastUser = ""
		return err
	}

	// pack the data into an audio.Msg which is used for further internal
	// processing
	audioMsg := audio.Msg{
		Channels:   channels,
		Data:       buf,
		EOF:        false,
		Frames:     num,
		Samplerate: float64(msg.GetSamplingRate()), // we want 48kHz for internal processing
		Metadata:   map[string]interface{}{"userID": msg.GetUserId()},
	}

	pbr.callback(audioMsg)

	return nil
}

func newOpusDecoder(channels int) (*opus.OpusDecoder, error) {
	decChannels := opus.Channels(channels)
	decSR := opus.Samplerate(48000) // opus only likes 48kHz
	dec, err := opus.NewOpusDecoder(decChannels, decSR)

	return dec, err
}
