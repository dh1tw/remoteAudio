package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	// _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audio/scReader"
	"github.com/dh1tw/remoteAudio/audio/scWriter"
	"github.com/dh1tw/remoteAudio/audio/wavReader"
	"github.com/dh1tw/remoteAudio/audio/wavWriter"
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
			fmt.Println("no config file f ound")
		} else {
			fmt.Println("Error parsing config file", viper.ConfigFileUsed())
			fmt.Println(err)
			os.Exit(1)
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
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

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

	r, err := audio.NewRouter()
	if err != nil {
		log.Fatal(err)
	}

	s, err := audio.NewSelector()
	if err != nil {
		log.Fatal(err)
	}

	n := &natsServer{
		router:   r,
		selector: s,
	}

	speaker, err := scWriter.NewScWriter(
		scWriter.DeviceName(oDeviceName),
		scWriter.Channels(oChannels),
		scWriter.Samplerate(oSamplerate),
		scWriter.Latency(oLatency),
		scWriter.RingBufferSize(oRingBufferSize),
		scWriter.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	wavRec, err := wavWriter.NewWavWriter("test_rec.wav",
		wavWriter.BitDepth(8),
		wavWriter.Channels(1),
		wavWriter.Samplerate(4000))
	if err != nil {
		log.Fatal(err)
	}

	n.router.AddSink("speaker", speaker, false)
	n.router.AddSink("wavFile", wavRec, false)

	wav, err := wavReader.NewWavReader("./test.wav")
	if err != nil {
		log.Fatal(err)
	}

	mic, err := scReader.NewScReader(
		scReader.Callback(n.recCb),
		scReader.DeviceName(iDeviceName),
		scReader.Channels(iChannels),
		scReader.Samplerate(iSamplerate),
		scReader.Latency(iLatency),
		scReader.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	n.selector.AddSource("mic", mic)
	n.selector.AddSource("file", wav)

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	n.selector.SetCb(n.recCb)
	n.router.EnableSink("speaker", true)
	n.selector.SetSource("mic")

	keyb := make(chan string, 10)

	go func() {
		for {
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			keyb <- strings.TrimSuffix(text, "\n")
		}
	}()

	for {
		select {
		case input := <-keyb:
			switch input {
			case "a":
				if err := n.router.EnableSink("wavFile", true); err != nil {
					log.Println(err)
				}
			case "b":
				if err := n.router.EnableSink("wavFile", false); err != nil {
					log.Println(err)
				}
			case "f":
				n.router.Flush()
				if err := n.selector.SetSource("file"); err != nil {
					log.Println(err)
				}
			case "m":
				n.router.Flush()
				if err := n.selector.SetSource("mic"); err != nil {
					log.Println(err)
				}
			case "i":
				speaker.SetVolume(speaker.Volume() + 0.5)
			case "d":
				speaker.SetVolume(speaker.Volume() - 0.5)
			}
		case sig := <-osSignals:
			if sig == os.Interrupt {
				// TBD: close also router (and all sinks)
				mic.Close()
				wavRec.Close()
				return
			}
		}
	}
}

type natsServer struct {
	router    audio.Router
	selector  audio.Selector
	isPlaying bool
	play      chan audio.AudioMsg
}

func (n *natsServer) recCb(data audio.AudioMsg) {
	token := n.router.Enqueue(data)
	token.Wait()
	if data.EOF {
		// switch back to default source
		n.router.Flush()
		if err := n.selector.SetSource("mic"); err != nil {
			log.Println(err)
		}
	}
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
