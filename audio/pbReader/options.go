package pbReader

import (
	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audiocodec"
)

// Option is the type for a function option
type Option func(*Options)

type Options struct {
	DeviceName string
	Decoder    audiocodec.Decoder
	Callback   audio.OnDataCb
}
