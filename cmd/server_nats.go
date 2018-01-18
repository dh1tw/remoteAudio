package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"net/http"
	// _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/router"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverMqttCmd represents the mqtt command
var natsServerCmd = &cobra.Command{
	Use:   "nats",
	Short: "nats",
	Long:  `nats`,
	Run:   natsAudioServer,
}

func init() {
	serverCmd.AddCommand(natsServerCmd)
	natsServerCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	natsServerCmd.Flags().IntP("broker-port", "p", 4222, "Broker Port")
	natsServerCmd.Flags().StringP("password", "P", "", "NATS Password")
	natsServerCmd.Flags().StringP("username", "U", "", "NATS Username")
	natsServerCmd.Flags().StringP("radio", "Y", "myradio", "Radio ID")
}

func natsAudioServer(cmd *cobra.Command, args []string) {

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if strings.Contains(err.Error(), "Not Found in") {
			fmt.Println("no config file found")
		} else {
			fmt.Println("Error parsing config file", viper.ConfigFileUsed())
			fmt.Println(err)
			os.Exit(-1)
		}
	}

	// check if values from config file / pflags are valid
	if !checkAudioParameterValues() {
		os.Exit(-1)
	}

	// bind the pflags to viper settings
	viper.BindPFlag("nats.broker-url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("nats.broker-port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("nats.password", cmd.Flags().Lookup("password"))
	viper.BindPFlag("nats.username", cmd.Flags().Lookup("username"))
	viper.BindPFlag("nats.radio", cmd.Flags().Lookup("radio"))

	// profiling server
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant

	audioFramesPerBuffer := viper.GetInt("audio.frame-length")

	oDeviceName := viper.GetString("output-device.device-name")
	oSamplerate := viper.GetFloat64("output-device.samplerate")
	oLatency := viper.GetDuration("output-device.latency")
	oChannels := viper.GetInt("output-device.channels")
	oRingBufferSize := viper.GetInt("audio.rx-buffer-length")

	iDeviceName := viper.GetString("input-device.device-name")
	iSamplerate := viper.GetFloat64("input-device.samplerate")
	iLatency := viper.GetDuration("input-device.latency")
	iChannels := viper.GetInt("input-device.channels")

	portaudio.Initialize()
	defer portaudio.Terminate()

	r, err := router.NewRouter()
	if err != nil {
		log.Fatal(err)
	}

	n := &natsServer{
		Router: r,
		play:   make(chan audio.AudioMsg, 10),
	}

	player, err := audio.NewPlayer(
		audio.DeviceName(oDeviceName),
		audio.Channels(oChannels),
		audio.Samplerate(oSamplerate),
		audio.Latency(oLatency),
		audio.RingBufferSize(oRingBufferSize),
		audio.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	rec, err := audio.NewRecorder(n.recCb,
		audio.DeviceName(iDeviceName),
		audio.Channels(iChannels),
		audio.Samplerate(iSamplerate),
		audio.Latency(iLatency),
		audio.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	n.AddSink("player", player, true)

	wav, err := audio.WavFile("./test.wav")
	if err != nil {
		log.Fatal(err)
	}

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	if err := player.Start(); err != nil {
		log.Fatal(err)
	}

	if err := rec.Start(); err != nil {
		log.Fatal(err)
	}

	keyb := make(chan string, 10)

	go func() {
		for {
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			keyb <- strings.TrimSuffix(text, "\n")
		}
	}()

	stop := make(chan struct{})
	finish := make(chan bool)

	for {
		select {
		case input := <-keyb:
			switch input {
			case "a":
				rec.Start()
			case "b":
				if n.isPlaying {
					continue
				}
				rec.Stop()
				n.Flush()
				n.isPlaying = true
				stop = n.PlayAudio(wav, finish)
			case "c":
				if n.isPlaying {
					close(stop)
				}
			case "i":
				player.SetVolume(player.Volume() + 0.5)
			case "d":
				player.SetVolume(player.Volume() - 0.5)
			}
		case msg := <-n.play:
			if n.isPlaying {
				continue
			}
			n.Enqueue(msg)
		case <-finish:
			n.Flush()
			n.isPlaying = false
			rec.Start()
		case sig := <-osSignals:
			if sig == os.Interrupt {
				// TBD: close also router (and all sinks)
				rec.Close()
				return
			}
		}
	}
}

func (n *natsServer) PlayAudio(msgs []audio.AudioMsg, finishCh chan<- bool) chan struct{} {
	fmt.Println("playing sound")

	stopCh := make(chan struct{})

	// play the audio in a seperate go routine which allows us to
	// cancel at any time by closing the stopCh
	// the closure of finishCh signals that the playing has finished
	go func() {
		defer func() { finishCh <- true }()
		for i := 0; i < len(msgs); i++ {
			select {
			case <-stopCh:
				return
			default:
				token := n.Router.Enqueue(msgs[i])
				token.Wait()
			}
		}
	}()

	return stopCh
}

type natsServer struct {
	router.Router
	isPlaying bool
	play      chan audio.AudioMsg
}

func (n *natsServer) recCb(data audio.AudioMsg) {
	n.play <- data
}

// // GetOpusApplication returns the integer representation of a
// // Opus application value string (typically read from application settings)
// func GetOpusApplication(app string) (opus.Application, error) {
// 	switch app {
// 	case "audio":
// 		return opus.AppAudio, nil
// 	case "restricted_lowdelay":
// 		return opus.AppRestrictedLowdelay, nil
// 	case "voip":
// 		return opus.AppVoIP, nil
// 	}
// 	return 0, errors.New("unknown opus application value")
// }

// // GetOpusMaxBandwith returns the integer representation of an
// // Opus max bandwitdh value string (typically read from application settings)
// func GetOpusMaxBandwith(maxBw string) (opus.Bandwidth, error) {
// 	switch strings.ToLower(maxBw) {
// 	case "narrowband":
// 		return opus.Narrowband, nil
// 	case "mediumband":
// 		return opus.Mediumband, nil
// 	case "wideband":
// 		return opus.Wideband, nil
// 	case "superwideband":
// 		return opus.SuperWideband, nil
// 	case "fullband":
// 		return opus.Fullband, nil
// 	}

// 	return 0, errors.New("unknown opus max bandwidth value")
// }

// // GetCodec return the integer representation of a string containing the
// // name of an audio codec
// func GetCodec(codec string) (int, error) {
// 	switch strings.ToLower(codec) {
// 	case "pcm":
// 		return PCM, nil
// 	case "opus":
// 		return OPUS, nil
// 	}
// 	errMsg := fmt.Sprintf("unknown codec: %s", codec)
// 	return 0, errors.New(errMsg)
// }

// var bitMapToInt32 = map[int32]float32{
// 	8:  255,
// 	12: 4095,
// 	16: 32767,
// 	32: 2147483647,
// }

// var bitMapToFloat32 = map[int]float32{
// 	8:  256,
// 	12: 4096,
// 	16: 32768,
// 	32: 2147483648,
// }
