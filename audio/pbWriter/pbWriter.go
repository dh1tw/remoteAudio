package pbWriter

import (
	"fmt"
	"sync"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec"
	"github.com/gogo/protobuf/proto"

	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
)

type PbWriter struct {
	sync.RWMutex
	options Options
	enabled bool
	encoder audiocodec.Encoder
	cb      func([]byte)
}

func NewPbWriter(cb func([]byte), opts ...Option) (*PbWriter, error) {

	enc, err := opus.NewOpusEncoder()
	if err != nil {
		return nil, err
	}

	pbw := &PbWriter{
		encoder: enc,
		options: Options{
			DeviceName: "ProtoBufReader",
		},
		cb: cb,
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

	// check if channels, Frames number, Samplerate correspond with codec

	buf := make([]byte, 100000)

	num, err := pbw.encoder.Encode(audioMsg.Data, buf)
	if err != nil {
		fmt.Println(err)
	}

	msg := sbAudio.Frame{
		Data:         buf[:num],
		Channels:     sbAudio.Channels_stereo,
		BitDepth:     16,
		Codec:        sbAudio.Codec_opus,
		FrameLength:  int32(audioMsg.Frames),
		SamplingRate: int32(audioMsg.Samplerate),
		UserId:       "dh1tw",
	}

	data, err := proto.Marshal(&msg)
	if err != nil {
		return err
	}

	pbw.cb(data)
	fmt.Println(data)

	return nil
}

func (pbw *PbWriter) Flush() {

}
