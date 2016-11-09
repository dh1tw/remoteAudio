package audio

import (
	"reflect"
	"testing"
)

func TestSerializeDeserialize(t *testing.T) {

	audiodata := []int32{5, -5, 12, 4, 48, 12}

	data, err := serializeAudioMsg(4800, 1024, 1, audiodata)

	if err != nil {
		t.Fatal(err)
	}

	msg, err := deserializeAudioMsg(data)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(audiodata, msg.Data) {
		t.Log("serialized:", audiodata)
		t.Log("deserialized:", msg.Data)
		t.Fatal("serialized & deserialized data not equal")
	}
}
