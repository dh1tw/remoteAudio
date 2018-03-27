package pbWriter

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	"github.com/gogo/protobuf/proto"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
)

type PbWriter struct {
	sync.RWMutex
	options Options
	enabled bool
	buffer  []byte
	cb      func([]byte)
}

func NewPbWriter(cb func([]byte), opts ...Option) (*PbWriter, error) {

	pbw := &PbWriter{
		options: Options{
			DeviceName: "ProtoBufReader",
			Channels:   2,
			Samplerate: 48000,
		},
		buffer: make([]byte, 10000),
		cb:     cb,
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

	return pbw, nil
}

func (pbw *PbWriter) Start() error {
	return nil
}

func (pbw *PbWriter) Stop() error {
	return nil
}

func (pbw *PbWriter) Close() error {
	return nil
}

func (pbw *PbWriter) SetVolume(vol float32) {

}

func (pbw *PbWriter) Volume() float32 {
	return 1
}

func (pbw *PbWriter) Write(audioMsg audio.Msg, token audio.Token) error {

	if pbw.cb == nil {
		return nil
	}

	if pbw.options.Encoder == nil {
		log.Println("no encoder set")
		return errors.New("no encoder set")
	}

	// check if channels, Frames number, Samplerate correspond with codec

	num, err := pbw.options.Encoder.Encode(audioMsg.Data, pbw.buffer)
	if err != nil {
		fmt.Println(err)
	}

	channels := sbAudio.Channels_unknown
	switch audioMsg.Channels {
	case 1:
		channels = sbAudio.Channels_mono
	case 2:
		channels = sbAudio.Channels_stereo
	}

	msg := sbAudio.Frame{
		Data:         pbw.buffer[:num],
		Channels:     channels,
		BitDepth:     16,
		Codec:        sbAudio.Codec_opus,
		FrameLength:  int32(audioMsg.Frames),
		SamplingRate: int32(pbw.options.Samplerate),
		UserId:       "dh1tw",
	}

	data, err := proto.Marshal(&msg)
	if err != nil {
		return err
	}

	pbw.cb(data)

	return nil
}

func (pbw *PbWriter) Flush() {

}
