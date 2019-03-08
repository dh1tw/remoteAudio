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
	"github.com/gogo/protobuf/proto"
)

// PbReader implements the audio.Source interface and is used to read
// audio frames encapsulated in Protocol Buffer messages, typically
// received from the network.
type PbReader struct {
	sync.RWMutex
	options               Options
	enabled               bool
	txUser                string
	lastPacket            time.Time
	notifyTxUserChangedCb func()
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

func (pbr *PbReader) Start() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = true
	// go pbr.checkTxUser()
	return nil
}

func (pbr *PbReader) Stop() error {
	pbr.Lock()
	defer pbr.Unlock()
	pbr.enabled = false
	return nil
}

// func (pbr *PbReader) checkTxUser() {
// 	for {
// 		select {
// 		case <-time.After(time.Millisecond * 100):
// 			pbr.RLock()
// 			if time.Since(pbr.lastPacket) > time.Millisecond*100 {
// 				pbr.txUser = ""
// 			}
// 			pbr.RUnlock()
// 		}
// 	}
// }

// func (pbr *PbReader) notifyTxUserChanged() {

// 	if pbr.notifyTxUserChangedCb == nil{
// 		return
// 	}
// 	pbr.notifyTxUserChangedCb(pbr.txUser)
// }

// func (pbr *PbReader) SetTxUserChangedCb(cb func()) {
// 	pbr.Lock()
// 	defer pbr.Unlock()
// 	pbr.notifyTxUserChangedCb = cb
// }

func (pbr *PbReader) Close() error {
	return nil
}

func (pbr *PbReader) SetCb(cb audio.OnDataCb) {
	pbr.Lock()
	defer pbr.Unlock()
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
		Metadata:   map[string]interface{}{"origin": msg.GetUserId()},
	}

	pbr.options.Callback(audioMsg)

	return nil
}
