package audio

import (
	"encoding/binary"

	"github.com/dh1tw/remoteAudio/icd"
	"github.com/golang/protobuf/proto"
)

func (ad *AudioDevice) serializeAudioMsg(samples []int32) ([]byte, error) {

	f := int32(ad.FramesPerBuffer)
	s := int32(ad.Samplingrate)
	c := int32(ad.Channels)
	d := make([]byte, 0, 4*len(samples))

	temp := make([]byte, 4)

	for _, sample := range samples {
		binary.LittleEndian.PutUint32(temp, uint32(sample))
		d = append(d, temp...)
	}

	msg := icd.AudioData{
		Channels:     &c,
		FrameLength:  &f,
		SamplingRate: &s,
		Audio:        d,
	}

	data, err := proto.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return data, nil
}
