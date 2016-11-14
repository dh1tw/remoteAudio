package audio

import (
	"reflect"
	"testing"
)

func TestSerializeDeserialize32Bit(t *testing.T) {

	ad := AudioDevice{}
	ad.Bitrate = 32
	ad.Samplingrate = 48000
	ad.FramesPerBuffer = 8
	ad.Channels = 1
	ad.in.Data32 = []int32{5, -5, 12, 4, 48, 12, 37, -12}
	data, err := ad.serializeAudioMsg()

	if err != nil {
		t.Fatal(err)
	}

	adIn := AudioDevice{}
	adIn.Bitrate = 32
	adIn.Samplingrate = 48000
	adIn.FramesPerBuffer = 8
	adIn.Channels = 1
	adIn.out.Data32 = make([]int32, 8)

	err = adIn.deserializeAudioMsg(data)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ad.in, adIn.out) {
		t.Log("serialized byte array", data)
		t.Log("serialized:", ad.in)
		t.Log("deserialized:", ad.out)
		t.Fatal("serialized & deserialized data not equal")
	}
}

func TestSerializeDeserialize16Bit(t *testing.T) {

	ad := AudioDevice{}
	ad.Bitrate = 16
	ad.Samplingrate = 48000
	ad.FramesPerBuffer = 8
	ad.Channels = 1
	ad.in.Data16 = []int16{5, -5, 12, 4, 48, 12, 37, -12}
	data, err := ad.serializeAudioMsg()

	if err != nil {
		t.Fatal(err)
	}

	adIn := AudioDevice{}
	adIn.Bitrate = 16
	adIn.Samplingrate = 48000
	adIn.FramesPerBuffer = 8
	adIn.Channels = 1
	adIn.out.Data16 = make([]int16, 8)

	err = adIn.deserializeAudioMsg(data)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ad.in, adIn.out) {
		t.Log("serialized byte array", data)
		t.Log("serialized:", ad.in)
		t.Log("deserialized:", ad.out)
		t.Fatal("serialized & deserialized data not equal")
	}
}

func TestSerializeDeserialize8Bit(t *testing.T) {

	ad := AudioDevice{}
	ad.Bitrate = 8
	ad.Samplingrate = 48000
	ad.FramesPerBuffer = 8
	ad.Channels = 1
	ad.in.Data8 = []int8{5, -5, 12, 4, 48, 12, 37, -12}
	data, err := ad.serializeAudioMsg()

	if err != nil {
		t.Fatal(err)
	}

	adIn := AudioDevice{}
	adIn.Bitrate = 8
	adIn.Samplingrate = 48000
	adIn.FramesPerBuffer = 8
	adIn.Channels = 1
	adIn.out.Data8 = make([]int8, 8)

	err = adIn.deserializeAudioMsg(data)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ad.in, adIn.out) {
		t.Log("serialized byte array", data)
		t.Log("serialized:", ad.in)
		t.Log("deserialized:", ad.out)
		t.Fatal("serialized & deserialized data not equal")
	}
}
