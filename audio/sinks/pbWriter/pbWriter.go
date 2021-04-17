package pbWriter

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	"github.com/golang/protobuf/proto"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
)

// PbWriter implements the audio.Sink interface. It is used to encode
// audio.Msg with a selected audiocodec into Protocol buffers. This
// Sink is typically used when audio frames have to be send to the network.
type PbWriter struct {
	sync.RWMutex
	options Options
	enabled bool
	buffer  []byte
	stash   []float32
	src     src
	volume  float32
}

// src contains a samplerate converter and its needed variables
type src struct {
	gosamplerate.Src
	samplerate float64
	ratio      float64
}

// NewPbWriter is the constructor for a ProtoBufWriter. The ToWireCb callback will
// be executed when an audio.Msg has been encoded into a protobuf byte slice and
// ready to be send to the network. Additional functional options can be passed
// in (e.g. the audio codec to be used).
func NewPbWriter(opts ...Option) (*PbWriter, error) {

	pbw := &PbWriter{
		options: Options{
			DeviceName:      "ProtoBufReader",
			Channels:        1,
			Samplerate:      48000,
			FramesPerBuffer: 960,
			UserID:          "myCallsign",
		},
		buffer: make([]byte, 10000),
		volume: 0.7,
	}

	for _, option := range opts {
		option(&pbw.options)
	}

	// if no encoder set, create the default encoder
	if pbw.options.Encoder == nil {
		encChannels := opus.Channels(pbw.options.Channels)
		encSR := opus.Samplerate(pbw.options.Samplerate)
		enc, err := opus.NewEncoder(encChannels, encSR)
		if err != nil {
			return nil, err
		}
		pbw.options.Encoder = enc
	}

	// setup a samplerate converter
	srConv, err := gosamplerate.New(gosamplerate.SRC_SINC_FASTEST,
		pbw.options.Channels, 65536)
	if err != nil {
		return nil, fmt.Errorf("PbWriter samplerate converter: %v", err)
	}
	pbw.src = src{
		Src:        srConv,
		samplerate: pbw.options.Samplerate,
		ratio:      1,
	}

	return pbw, nil
}

// Start starts this audio sink.
func (pbw *PbWriter) Start() error {
	pbw.Lock()
	defer pbw.Unlock()
	pbw.enabled = true
	return nil
}

// Stop disables this audio sink.
func (pbw *PbWriter) Stop() error {
	pbw.Lock()
	defer pbw.Unlock()
	pbw.enabled = false
	return nil
}

// Close is not implemented
func (pbw *PbWriter) Close() error {
	return nil
}

// SetVolume sets the volume for this sink
func (pbw *PbWriter) SetVolume(v float32) {
	pbw.Lock()
	defer pbw.Unlock()
	if v < 0 {
		pbw.volume = 0
	} else if v > 1 {
		pbw.volume = 1
	} else {
		pbw.volume = v
	}
}

// Volume returns the volume for this sink
func (pbw *PbWriter) Volume() float32 {
	pbw.RLock()
	defer pbw.RUnlock()
	return pbw.volume
}

// Write is called to encode audio.Msg with a specified audio codec into
// protobufs. The Token is not used. On success the protobuf ([]byte) will
// be returned in a callback.
func (pbw *PbWriter) Write(audioMsg audio.Msg) error {

	pbw.Lock()
	defer pbw.Unlock()

	if !pbw.enabled {
		return nil
	}

	if pbw.options.ToWireCb == nil {
		return nil
	}

	if pbw.options.Encoder == nil {
		return errors.New("no encoder set")
	}

	// The resampling and encoding can be quite expensive (e.g. with opus). Therefore it is
	// launched in a separate go routine.
	go func() {

		pbw.Lock()
		defer pbw.Unlock()

		var aData []float32
		var err error

		// if necessary adjust the amount of audio channels
		if audioMsg.Channels != pbw.options.Channels {
			aData = audio.AdjustChannels(audioMsg.Channels,
				pbw.options.Channels, audioMsg.Data)
		} else {
			aData = audioMsg.Data
		}

		if audioMsg.Samplerate != pbw.options.Samplerate {
			if pbw.src.samplerate != audioMsg.Samplerate {
				pbw.src.Reset()
				pbw.src.samplerate = audioMsg.Samplerate
				pbw.src.ratio = pbw.options.Samplerate / audioMsg.Samplerate
			}
			aData, err = pbw.src.Process(aData, pbw.src.ratio, false)
			if err != nil {
				log.Println(err)
				return
			}
		}

		// audio buffer size we want to push into the opus encuder
		// opus only allows certain buffer sizes (2,5ms, 5ms, 10ms...etc)
		expBufferSize := pbw.options.Channels * pbw.options.FramesPerBuffer

		// if there is data stashed from previous calles, get it and prepend it
		// to the data received
		if len(pbw.stash) > 0 {
			aData = append(pbw.stash, aData...)
			pbw.stash = pbw.stash[:0] // empty
		}

		// if audioMsg.EOF {
		// 	// get the stuff from the stash
		// 	fmt.Println("EOF!!!")
		// 	fmt.Println("stash size:", len(pbw.stash))
		// }

		// if the audio buffer size is actually smaller than the one we need,
		// then stash it for the next time and return
		if len(aData) < expBufferSize {
			pbw.stash = aData
			return
		}

		// slice of audio buffers which will be send
		var bData [][]float32

		// if the aData contains multiples of the expected buffer size,
		// then we chop it into (several) buffers
		if len(aData) >= expBufferSize {
			vol := pbw.volume

			for len(aData) >= expBufferSize {
				if vol != 1 {
					// if necessary, adjust the volume
					audio.AdjustVolume(vol, aData[:expBufferSize])
				}
				bData = append(bData, aData[:expBufferSize])
				aData = aData[expBufferSize:]
			}
		}

		// stash the left over
		if len(aData) > 0 {
			pbw.stash = aData
		}

		channels := sbAudio.Channels_unknown
		switch pbw.options.Channels {
		case 1:
			channels = sbAudio.Channels_mono
		case 2:
			channels = sbAudio.Channels_stereo
		}

		for _, frame := range bData {
			num, err := pbw.options.Encoder.Encode(frame, pbw.buffer)
			if err != nil {
				log.Println(err)
			}

			msg := sbAudio.Frame{
				Data:         pbw.buffer[:num],
				Channels:     channels,
				BitDepth:     16,
				Codec:        sbAudio.Codec_opus,
				FrameLength:  int32(pbw.options.FramesPerBuffer),
				SamplingRate: int32(pbw.options.Samplerate),
				UserId:       pbw.options.UserID,
			}

			data, err := proto.Marshal(&msg)
			if err != nil {
				log.Println(err)
				return
			}
			pbw.options.ToWireCb(data)
		}

	}()

	return nil
}

// Flush clears all internal buffers
func (pbw *PbWriter) Flush() {
	pbw.Lock()
	defer pbw.Unlock()

	pbw.stash = []float32{}
}

// SetToWireCb allows to set a callback which will be called whenever
// the data has been serialized and is ready to be send on the wire.
func (pbw *PbWriter) SetToWireCb(cb func([]byte)) {
	pbw.Lock()
	defer pbw.Unlock()

	pbw.options.ToWireCb = cb
}
