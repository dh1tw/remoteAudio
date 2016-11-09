package audio

import (
	"encoding/binary"

	"github.com/dh1tw/remoteAudio/icd"
	"github.com/golang/protobuf/proto"
)

func deserializeAudioMsg(data []byte) (AudioSamples, error) {

	msg := icd.AudioData{}
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return AudioSamples{}, err
	}

	audioData := make([]int32, 0, len(msg.Audio)/4)

	if msg.Audio != nil {
		for i := 0; i < len(msg.Audio); i = i + 4 {
			sample := binary.LittleEndian.Uint32(msg.Audio[i : i+4])
			audioData = append(audioData, int32(sample))
		}
	}

	audioSamples := AudioSamples{
		Channels:     msg.GetChannels(),
		Samplingrate: msg.GetSamplingRate(),
		Frames:       msg.GetFrameLength(),
		Data:         audioData,
	}

	return audioSamples, nil
}
