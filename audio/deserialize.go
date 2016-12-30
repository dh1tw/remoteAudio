package audio

import (
	"errors"

	"github.com/dh1tw/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
)

type deserializer struct {
	*AudioDevice
	opusDecoder *opus.Decoder
	opusBuffer  []float32
}

func (d *deserializer) DeserializeAudioMsg(data []byte) error {
	msg := sbAudioDataPool.Get().(*sbAudio.AudioData)
	defer sbAudioDataPool.Put(msg)

	err := proto.Unmarshal(data, msg)
	if err != nil {
		return err
	}

	if msg.Codec == sbAudio.Codec_OPUS {
		err := d.DeserializeOpusAudioMsg(msg)
		if err != nil {
			return err
		}
	} else if msg.Codec == sbAudio.Codec_PCM {
		err := d.DeserializePCMAudioMsg(msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *deserializer) DeserializeOpusAudioMsg(msg *sbAudio.AudioData) error {

	len, err := d.opusDecoder.DecodeFloat32(msg.GetAudioRaw(), d.opusBuffer)
	if err != nil {
		return err
	}

	d.out = d.opusBuffer[:len*d.Channels]

	return nil
}

// DeserializeAudioMsg deserializes protocol buffers containing audio frames with
// the corresponding meta data. In case the Audio channels and / or Samplerate
// doesn't match with the hosts values, both will be converted to match.
func (ad *AudioDevice) DeserializePCMAudioMsg(msg *sbAudio.AudioData) error {

	var samplingrate float64
	var channels, bitdepth int

	channels = int(msg.GetChannels())
	if channels == 0 {
		return errors.New("invalid amount of channels")
	}

	samplingrate = float64(msg.GetSamplingRate())
	if samplingrate == 0 {
		return errors.New("invalid samplerate")
	}

	// only accept 8, 12, 16 or 32 bit streams
	bitdepth = int(msg.GetBitDepth())

	if bitdepth != 8 && bitdepth != 12 && bitdepth != 16 && bitdepth != 32 {
		return errors.New("incompatible audio bit depth")
	}

	if len(msg.AudioPacked) == 0 {
		return errors.New("empty audio buffer")
	}

	// convert the data to float32 (8bit, 12bit, 16bit, 32bit)
	convertedAudio := make([]float32, 0, len(msg.AudioPacked))
	for _, sample := range msg.AudioPacked {
		convertedAudio = append(convertedAudio, float32(sample)/bitMapToFloat32[bitdepth])
	}

	if msg.Codec == sbAudio.Codec_PCM {
		// if necessary, adjust the channels
		if channels != ad.Channels {

			// audio device is STEREO but we received MONO
			if channels == MONO && ad.Channels == STEREO {
				expanded := make([]float32, 0, len(convertedAudio)*2)
				// left channel = right channel
				for _, sample := range convertedAudio {
					expanded = append(expanded, sample)
					expanded = append(expanded, sample)
				}
				convertedAudio = expanded

			} else if channels == STEREO && ad.Channels == MONO {
				// audio device is MONO but we received STEREO
				reduced := make([]float32, 0, len(convertedAudio)/2)
				// chop of the right channel
				for i := 0; i < len(convertedAudio); i += 2 {
					reduced = append(reduced, convertedAudio[i])
				}
				convertedAudio = reduced
			}
		}

		var resampledAudio []float32
		var err error

		// if necessary, resample the audio
		if samplingrate != ad.Samplingrate {
			ratio := ad.Samplingrate / samplingrate // output samplerate / input samplerate
			resampledAudio, err = ad.Converter.Process(convertedAudio, ratio, false)
			if err != nil {
				return err
			}
			ad.out = resampledAudio
		} else {
			ad.out = convertedAudio
		}
	}
	return nil
}
