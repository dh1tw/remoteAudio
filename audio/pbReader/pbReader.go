package pbReader

import (
	"fmt"
	"sync"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
)

// PbReader implements the audio.Source interface and is used to read
// audio frames encapsulated in Protocol Buffer messages, typically
// received from the network.
type PbReader struct {
	sync.RWMutex
	options Options
	enabled bool
}

func NewPbReader(opts ...Option) (*PbReader, error) {

	dec, err := opus.NewOpusDecoder()
	if err != nil {
		return nil, err
	}

	pbr := &PbReader{
		options: Options{
			DeviceName: "ProtoBufReader",
			Decoder:    dec,
		},
	}

	return pbr, nil
}

func (pbr *PbReader) Start() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = true
	return nil
}

func (pbr *PbReader) Stop() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = false
	return nil
}

func (pbr *PbReader) Close() error {
	return nil
}

func (pbr *PbReader) SetCb(cb audio.OnDataCb) {
	pbr.options.Callback = cb
}

func (pbr *PbReader) Enqueue(data []byte) error {
	pbr.Lock()
	defer pbr.Unlock()

	if pbr.options.Callback == nil {
		return nil
	}

	if !pbr.enabled {
		return nil
	}

	msg := sbAudio.Frame{}
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return err
	}

	if msg.Codec.String() != pbr.options.Decoder.Name() {
		return fmt.Errorf("unknown codec %v", msg.Codec.String())
	}

	buf := make([]float32, 0, msg.FrameLength)

	num, err := pbr.options.Decoder.Decode(msg.Data, buf)
	if err != nil {
		return err
	}

	audioMsg := audio.Msg{
		Channels:   pbr.options.Decoder.Options().Channels,
		Data:       buf,
		EOF:        false,
		Frames:     num,
		IsStream:   true,
		Samplerate: float64(pbr.options.Decoder.Options().Samplerate),
	}

	pbr.options.Callback(audioMsg)

	return nil
}
