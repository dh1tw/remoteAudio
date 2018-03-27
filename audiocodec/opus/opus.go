package opus

import (
	"fmt"

	ac "github.com/dh1tw/remoteAudio/audiocodec"
	opus "gopkg.in/hraban/opus.v2"
)

type OpusDecoder struct {
	name    string
	options ac.Options
	decoder *opus.Decoder
}

type OpusEncoder struct {
	name        string
	options     ac.Options
	encoder     *opus.Encoder
	application opus.Application
}

// NewOpusDecoder is the constructor method for an Opus decoder.
func NewOpusDecoder(opts ...ac.Option) (*OpusDecoder, error) {

	oc := &OpusDecoder{
		name: "opus",
		options: ac.Options{
			Samplerate: 48000,
			Channels:   2,
		},
	}

	for _, option := range opts {
		option(&oc.options)
	}

	decoder, err := opus.NewDecoder(oc.options.Samplerate, oc.options.Channels)
	if err != nil {
		return nil, err
	}

	oc.decoder = decoder
	return oc, nil
}

// Name returns the name of the audio codec
func (oc *OpusDecoder) Name() string {
	return oc.name
}

// Options returns a copy of the codec's options
func (oc *OpusDecoder) Options() ac.Options {
	return oc.options
}

// Decode encoded Opus data into the supplied float32 buffer. On success, the
// number of samples written into the buffer will be returned.
func (oc *OpusDecoder) Decode(data []byte, pcm []float32, opts ...ac.Options) (int, error) {
	return oc.decoder.DecodeFloat32(data, pcm)
}

// NewOpusEncoder is the constructor method for an Opus encoder.
func NewOpusEncoder(opts ...ac.Option) (*OpusEncoder, error) {

	oEnc := &OpusEncoder{
		name:        "opus",
		application: opus.AppVoIP,
		options: ac.Options{
			Samplerate: 48000,
			Channels:   2,
		},
	}

	for _, option := range opts {
		option(&oEnc.options)
	}

	encoder, err := opus.NewEncoder(oEnc.options.Samplerate,
		oEnc.options.Channels,
		oEnc.application)

	if err != nil {
		return nil, err
	}

	oEnc.encoder = encoder
	return oEnc, nil
}

// Name returns the name of the audio codec
func (oEnc *OpusEncoder) Name() string {
	return oEnc.name
}

// Options returns a copy of the codec's options
func (oEnc *OpusEncoder) Options() ac.Options {
	return oEnc.options
}

// Encode either []float32 or []int16 with the opus codec into the supplied
// buffer. On success the amount of bytes written into the buffer will be returned.
func (oEnc *OpusEncoder) Encode(pcm interface{}, data []byte) (int, error) {
	switch v := pcm.(type) {
	case []float32:
		return oEnc.encoder.EncodeFloat32(pcm.([]float32), data)
	case []int16:
		return oEnc.encoder.Encode(pcm.([]int16), data)
	default:
		return 0, fmt.Errorf("can not encode type %v with opus codec", v)
	}
}
