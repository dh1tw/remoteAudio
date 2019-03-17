package opus

import (
	ac "github.com/dh1tw/remoteAudio/audiocodec"
	opus "gopkg.in/hraban/opus.v2"
)

// OpusDecoder is the data structure which holds internal values
// for the decoder.
type OpusDecoder struct {
	name    string
	options Options
	decoder *opus.Decoder
}

// NewOpusDecoder is the constructor method for an Opus decoder.
func NewOpusDecoder(opts ...Option) (*OpusDecoder, error) {

	oc := &OpusDecoder{
		name: "opus",
		options: Options{
			Samplerate: 48000,
			Channels:   2,
		},
	}

	for _, option := range opts {
		option(&oc.options)
	}

	decoder, err := opus.NewDecoder(int(oc.options.Samplerate),
		oc.options.Channels)

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
func (oc *OpusDecoder) Options() Options {
	return oc.options
}

// Decode encoded Opus data into the supplied float32 buffer. On success, the
// number of samples written into the buffer will be returned.
func (oc *OpusDecoder) Decode(data []byte, pcm []float32, opts ...ac.Options) (int, error) {
	return oc.decoder.DecodeFloat32(data, pcm)
}
