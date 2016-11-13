package audio

import (
	"github.com/dh1tw/remoteAudio/icd"
	"github.com/golang/protobuf/proto"
)

func (ad *AudioDevice) serializeAudioMsg() ([]byte, error) {

	f := int32(ad.FramesPerBuffer)
	s := int32(ad.Samplingrate)
	c := int32(ad.Channels)
	b := int32(ad.Bitrate)

	// d := make([]byte, 0, 2*len(ad.in.Data16))
	// d8 := make([]byte, 0, len(ad.in.Data8))

	// // 8 bit
	// data := make([]byte, 1)

	// // 16 bit
	// if b == 16 {
	// 	data = make([]byte, 2)
	// }

	// if b == 8 {
	// 	for _, sample := range ad.in.Data8 {
	// 		data[0] = uint8(sample)
	// 		d8 = append(d8, data...)
	// 	}
	// } else if b == 16 {
	// 	for _, sample := range ad.in.Data16 {
	// 		binary.LittleEndian.PutUint16(data, uint16(sample))
	// 		d16 = append(d16, data...)
	// 	}
	// }

	msg := icd.AudioData{}

	msg.Channels = &c
	msg.FrameLength = &f
	msg.SamplingRate = &s
	msg.Bitrate = &b
	msg.Audio = ad.in.Data32

	// if b == 16 {
	// 	msg.Audio = d16
	// } else if b == 8 {
	// 	msg.Audio = d8
	// }

	data, err := proto.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	return data, nil
}
