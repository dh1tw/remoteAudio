package audio

import (
	"fmt"

	"github.com/dh1tw/remoteAudio/icd"
	"github.com/golang/protobuf/proto"
	sox "github.com/krig/go-sox"
)

// Flow data from in to out via the samples buffer
func flow(in, out *sox.Format, samples []sox.Sample) {
	n := uint(len(samples))
	for number_read := in.Read(samples, n); number_read > 0; number_read = in.Read(samples, n) {
		out.Write(samples, uint(number_read))
	}
}

func (ad *AudioDevice) serializeAudioMsg() ([]byte, error) {

	f := int32(ad.FramesPerBuffer)
	s := int32(ad.Samplingrate)
	c := int32(ad.Channels)
	b := int32(ad.Bitrate)

	d32 := make([]int32, 0, len(ad.in.Data32))

	if b == 8 {
		// for _, sample := range ad.in.Data8 {
		// 	data[0] = uint8(sample)
		// 	d8 = append(d8, data...)
		// }
	} else if b == 16 {
		// for _, sample := range ad.in.Data16 {
		// 	binary.LittleEndian.PutUint16(data, uint16(sample))
		// 	d16 = append(d16, data...)
		// }
	} else if b == 32 {
		for _, sample := range ad.in.Data32 {
			d32 = append(d32, int32(sample*32767))
		}
	}

	msg := icd.AudioData{}

	msg.Channels = &c
	msg.FrameLength = &f
	msg.SamplingRate = &s
	msg.Bitrate = &b

	if b == 16 {
		// msg.Audio = d16

	} else if b == 8 {
		// msg.Audio = d8
		// buf32 := make([]int32, len(ad.in.Data8))
		// for i, sample := range ad.in.Data8 {
		// 	buf32[i] = int32(sample)
		// }
		// msg.Audio2 = buf32
	} else if b == 32 {
		msg.Audio2 = d32
		// msg.Audio2 = ad.in.Data32
	}

	data, err := proto.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	fmt.Println(len(data))

	return data, nil
}
