package audio

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/events"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
	ringBuffer "github.com/zfjagann/golang-ring"
	"gopkg.in/hraban/opus.v2"
)

type deserializer struct {
	*AudioDevice
	opusDecoder *opus.Decoder
	opusBuffer  []float32
	muTx        sync.Mutex
	txUser      string
	txTimestamp time.Time
	muRing      sync.Mutex
	ring        ringBuffer.Ring
}

// // deserialize and write received audio data into the ring buffer
// func (d *deserializer) Deserializer() {
// 	select {
// 	case msg := <-d.ToDeserialize:
// 		err := d.DeserializeAudioMsg(msg)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 	}
// }

// DeserializeAudioMsg will deserialize a Protocol Buffers message containing
// Audio data and its corresponding meta data
func (d *deserializer) DeserializeAudioMsg(data []byte) error {

	// using sync.Pool for releasing pressure of the Garbage Collector
	msg := sbAudioDataPool.Get().(*sbAudio.AudioData)
	defer sbAudioDataPool.Put(msg)

	err := proto.Unmarshal(data, msg)
	if err != nil {
		return err
	}

	// let's make sure that we only accept audio data from the correct user
	d.muTx.Lock()
	txUser := msg.GetUserId()

	switch d.txUser {
	// Nobody is currently transmitting
	case "":
		d.txUser = txUser
		d.txTimestamp = time.Now()
		d.muTx.Unlock()
	// tx is assigned to a user; only accept new audio data from him
	case txUser:
		d.txTimestamp = time.Now()
		d.muTx.Unlock()
	// return because someone else is transmitting
	default:
		errMsg := fmt.Sprintf("%s tries to send; however tx blocked by %s",
			txUser, d.txUser)
		d.muTx.Unlock()
		return errors.New(errMsg)
	}

	// select the codec (if field has not been set, it will default to OPUS)
	switch msg.GetCodec() {
	case sbAudio.Codec_OPUS:
		err = d.DecodeOpusAudioMsg(msg)
	case sbAudio.Codec_PCM:
		err = d.DecodePCMAudioMsg(msg)
	default:
		return errors.New("unknown Audio Codec")
	}
	if err != nil {
		return err
	}
	return nil
}

// DecodeOpusAudioMsg decodes a byte array containing an opus
// encoded audio frame.
func (d *deserializer) DecodeOpusAudioMsg(msg *sbAudio.AudioData) error {

	lenSample, err := d.opusDecoder.DecodeFloat32(msg.GetAudioRaw(), d.opusBuffer)
	if err != nil {
		return err
	}

	lenFrame := lenSample * d.Channels

	if lenFrame != len(d.out) {
		d.Events.Pub(lenFrame, events.NewAudioFrameSize)
	}

	//make a new array and copy the data into the array
	buf := make([]float32, lenFrame)
	for i := 0; i < lenFrame; i++ {
		buf[i] = d.opusBuffer[i]
	}

	d.muRing.Lock()
	d.ring.Enqueue(buf)
	d.muRing.Unlock()

	return nil
}

// DecodePCMAudioMsg conditions an int32 PCM audio frame according to the
// needs of the local audio stream (channels and/or sampling rate)
func (ad *deserializer) DecodePCMAudioMsg(msg *sbAudio.AudioData) error {

	var samplingrate float64
	var channels, bitdepth int

	channels = int(msg.GetChannels())
	if channels < 1 || channels > 2 {
		return errors.New("invalid amount of channels")
	}

	samplingrate = float64(msg.GetSamplingRate())
	if samplingrate <= 0 || samplingrate > 96000 {
		return errors.New("invalid samplerate")
	}

	bitdepth = int(msg.GetBitDepth())
	// only accept 8, 12, 16 or 32 bit streams
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

	// if necessary, adjust the channels to the local audio device channels
	if channels != ad.Channels {

		// case: local audio device is STEREO but we received MONO frame
		if channels == MONO && ad.Channels == STEREO {
			expanded := make([]float32, 0, len(convertedAudio)*2)
			// left channel = right channel
			for _, sample := range convertedAudio {
				expanded = append(expanded, sample)
				expanded = append(expanded, sample)
			}
			convertedAudio = expanded

			// case: local audio device is MONO but we received a STEREO frame
		} else if channels == STEREO && ad.Channels == MONO {
			reduced := make([]float32, 0, len(convertedAudio)/2)
			// chop off the right channel
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
		resampledAudio, err = ad.PCMSamplerateConverter.Process(convertedAudio, ratio, false)
		if err != nil {
			return err
		}

		ad.muRing.Lock()
		ad.ring.Enqueue(resampledAudio)
		ad.muRing.Unlock()
		// ad.out = resampledAudio
	} else {
		// ad.out = convertedAudio
		ad.muRing.Lock()
		ad.ring.Enqueue(convertedAudio)
		ad.muRing.Unlock()
	}

	return nil
}
