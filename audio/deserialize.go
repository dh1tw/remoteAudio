package audio

import (
	"errors"

	"github.com/dh1tw/remoteAudio/icd"
	"github.com/golang/protobuf/proto"
)

func (ad *AudioDevice) deserializeAudioMsg(data []byte) error {

	msg := icd.AudioData{}
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return err
	}

	// var channels, samplingrate, bitrate, frames int
	var channels, samplingrate, bitrate int

	if msg.Channels != nil {
		channels = int(msg.GetChannels())
	}

	if channels != ad.AudioStream.Channels {
		return errors.New("unequal Channels")
	}

	// if msg.FrameLength != nil {
	// 	frames = int(msg.GetFrameLength())
	// }

	if msg.String != nil {
		samplingrate = int(msg.GetSamplingRate())
	}

	if float64(samplingrate) != ad.AudioStream.Samplingrate {
		return errors.New("unequal Samplingrate")
	}

	// only accept 8 or 16 bit streams
	if msg.Bitrate != nil {
		bitrate = int(msg.GetBitrate())
		if bitrate != 8 && bitrate != 16 && bitrate != 32 {
			return errors.New("incompatible bitrate")
		}
	} else {
		return errors.New("unknown bitrate")
	}

	if len(msg.Audio) > 0 {
		ad.out.Data32 = msg.Audio
	} else {
		return errors.New("Audio stream empty")
	}

	// if bitrate == 8 {
	// 	if len(msg.Audio) != int(frames*channels) {
	// 		fmt.Println("msg length: ", len(msg.Audio), int(frames*channels), frames*channels)
	// 		return errors.New("audio data does not match frame buffer * channels")
	// 	}
	// } else if bitrate == 16 {
	// 	if len(msg.Audio) != int(frames*channels)*2 {
	// 		fmt.Println("msg length: ", len(msg.Audio), int(frames*channels), frames*channels)
	// 		return errors.New("audio data does not match frame buffer * channels")
	// 	}
	// }

	// if float64(samplingrate) != ad.Samplingrate {
	// 	return errors.New("unequal sampling rate")
	// }

	// if msg.Audio != nil {
	// 	if bitrate == 16 {
	// 		for i := 0; i < len(msg.Audio)/2; i++ {
	// 			sample := binary.LittleEndian.Uint16(msg.Audio[i*2 : i*2+2])
	// 			ad.out.Data16[i] = int16(sample)
	// 		}
	// 	} else if bitrate == 8 {
	// 		for i := 0; i < len(msg.Audio); i++ {
	// 			ad.out.Data8[i] = int8(msg.Audio[i])
	// 		}
	// 	}
	// }
	return nil
}
