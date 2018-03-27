package pbWriter

import (
	"github.com/dh1tw/remoteAudio/audiocodec"
)

// Option is the type for a function option
type Option func(*Options)

type Options struct {
	DeviceName string
	Encoder    audiocodec.Encoder
}
