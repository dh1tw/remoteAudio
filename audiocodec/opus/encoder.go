package opus

import (
	"fmt"

	opus "gopkg.in/hraban/opus.v2"
)

//OpusEncoder is the data structure for the opus encoder. This struct hold
//the internal values of the encoder.
type OpusEncoder struct {
	name        string
	options     Options
	encoder     *opus.Encoder
	application opus.Application
}

// NewEncoder is the constructor method for an Opus encoder.
func NewEncoder(opts ...Option) (*OpusEncoder, error) {

	oEnc := &OpusEncoder{
		name: "opus",
		options: Options{
			Samplerate:   48000,
			Channels:     1,
			MaxBandwidth: opus.Wideband,
			Application:  opus.AppRestrictedLowdelay,
			Bitrate:      24000,
			Complexity:   5,
		},
	}

	for _, option := range opts {
		option(&oEnc.options)
	}

	encoder, err := opus.NewEncoder(int(oEnc.options.Samplerate),
		oEnc.options.Channels,
		oEnc.options.Application)

	if err != nil {
		return nil, err
	}

	if err := encoder.SetBitrate(oEnc.options.Bitrate); err != nil {
		return nil, err
	}

	if err := encoder.SetComplexity(oEnc.options.Complexity); err != nil {
		return nil, err
	}

	if err := encoder.SetMaxBandwidth(oEnc.options.MaxBandwidth); err != nil {
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
func (oEnc *OpusEncoder) Options() Options {
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
