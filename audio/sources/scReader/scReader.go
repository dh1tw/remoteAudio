package scReader

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio"
	pa "github.com/gordonklaus/portaudio"
)

// ScReader implements the audio.Source interface and is used to read (record)
// audio from a local sound card (e.g. microphone).
type ScReader struct {
	sync.RWMutex
	options    Options
	deviceInfo *pa.DeviceInfo
	stream     *pa.Stream
	cb         func(audio.Msg)
}

// NewScReader returns a soundcard reader which steams audio
// asynchronously from an a local audio device (e.g. a microphone).
func NewScReader(opts ...Option) (*ScReader, error) {

	if err := pa.Initialize(); err != nil {
		return nil, err
	}

	r := &ScReader{
		options: Options{
			HostAPI:         "default",
			DeviceName:      "default",
			Channels:        1,
			Samplerate:      48000,
			FramesPerBuffer: 480,
			Latency:         time.Millisecond * 10,
		},
		deviceInfo: nil,
	}

	for _, option := range opts {
		option(&r.options)
	}

	var hostAPI *pa.HostApiInfo

	if r.options.HostAPI == "default" {
		switch runtime.GOOS {
		case "windows":
			// try to use WASAPI since it provides lower latency than the
			// other windows audio apis
			ha, err := pa.HostApi(pa.WASAPI)
			if err != nil {
				// try to fallback to the default API
				ha, err = pa.DefaultHostApi()
				if err != nil {
					return nil, fmt.Errorf("unable to determine the default host api - please provide a specific host api")
				}
			}
			hostAPI = ha
		default:
			// all other OS
			ha, err := pa.DefaultHostApi()
			if err != nil {
				return nil, fmt.Errorf("unable to determine the default host api - please provide a specific host api")
			}
			hostAPI = ha
		}
	} else {
		ha, err := getHostAPI(r.options.HostAPI)
		if err != nil {
			return nil, err
		}
		hostAPI = ha
	}

	if r.options.DeviceName == "default" {
		r.deviceInfo = hostAPI.DefaultInputDevice
	} else {
		dev, err := getPaDevice(r.options.DeviceName, hostAPI)
		if err != nil {
			return nil, err
		}
		r.deviceInfo = dev
	}

	// setup Audio Stream
	streamDeviceParam := pa.StreamDeviceParameters{
		Device:   r.deviceInfo,
		Channels: r.options.Channels,
		Latency:  r.options.Latency,
	}

	streamParm := pa.StreamParameters{
		FramesPerBuffer: r.options.FramesPerBuffer,
		Input:           streamDeviceParam,
		SampleRate:      r.options.Samplerate,
	}

	stream, err := pa.OpenStream(streamParm, r.paReadCb)
	if err != nil {
		return nil,
			fmt.Errorf("unable to open recording audio stream on device %s: %s",
				r.deviceInfo.Name, err)
	}
	r.stream = stream

	log.Printf("input sound device: %s, HostAPI: %s\n", r.deviceInfo.Name, r.deviceInfo.HostApi.Name)
	return r, nil
}

// SetCb sets the callback which will be executed to provide audio buffers.
func (r *ScReader) SetCb(cb audio.OnDataCb) {
	r.cb = cb
}

// paReadCb is the callback which will be executed each time there is new
// data available on the stream
func (r *ScReader) paReadCb(in []float32,
	iTime pa.StreamCallbackTimeInfo,
	iFlags pa.StreamCallbackFlags) {

	if iFlags == pa.InputOverflow {
		log.Println("InputOverflow")
		return // data lost, move on!
	}

	if r.cb == nil {
		return
	}

	// a deep copy is necessary, since portaudio reuses the slice "in"
	buf := make([]float32, len(in))
	for i, v := range in {
		buf[i] = v
	}

	msg := audio.Msg{
		Data:       buf,
		Samplerate: r.options.Samplerate,
		Channels:   r.options.Channels,
		Frames:     r.options.FramesPerBuffer,
	}

	// execute the callback for further processing
	go r.cb(msg)
}

// Start will start streaming audio from a local soundcard device.
// The read audio buffers will be provided through the callback.
func (r *ScReader) Start() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Start()
}

// Stop stops streaming audio.
func (r *ScReader) Stop() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Stop()
}

// Close shutsdown properly the soundcard reader.
func (r *ScReader) Close() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	r.stream.Abort()
	r.stream.Stop()
	return nil
}

// getHostAPI takes the name of a supported portaudio host api and returns
// the corresponding portaudio hostApiInfo object
func getHostAPI(name string) (*pa.HostApiInfo, error) {

	var hostAPIType pa.HostApiType

	switch strings.ToLower(name) {
	case "indevelopment":
		hostAPIType = pa.InDevelopment
	case "directsound":
		hostAPIType = pa.DirectSound
	case "mme":
		hostAPIType = pa.MME
	case "asio":
		hostAPIType = pa.ASIO
	case "soundmanager":
		hostAPIType = pa.SoundManager
	case "coreaudio":
		hostAPIType = pa.CoreAudio
	case "oss":
		hostAPIType = pa.OSS
	case "alsa":
		hostAPIType = pa.ALSA
	case "al":
		hostAPIType = pa.AL
	case "beos":
		hostAPIType = pa.BeOS
	case "wdmks":
		hostAPIType = pa.WDMkS
	case "jack":
		hostAPIType = pa.JACK
	case "wasapi":
		hostAPIType = pa.WASAPI
	case "audiosciencehpi":
		hostAPIType = pa.AudioScienceHPI
	default:
		return nil, fmt.Errorf("unknown host api type: %s", name)
	}

	hostAPIInfo, err := pa.HostApi(hostAPIType)
	if err != nil {
		return nil, fmt.Errorf("unable to load host api %s: %s", name, err.Error())
	}

	return hostAPIInfo, nil

}

// getPaDevice checks if the Audio Devices actually exist and
// then returns it
func getPaDevice(name string, hostAPI *pa.HostApiInfo) (*pa.DeviceInfo, error) {
	for _, device := range hostAPI.Devices {
		if strings.ToLower(device.Name) == strings.ToLower(name) {
			return device, nil
		}
	}
	return nil, fmt.Errorf("unknown audio device '%s'", name)
}
