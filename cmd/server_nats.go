package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	// _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/pbReader"
	"github.com/dh1tw/remoteAudio/audio/pbWriter"
	"github.com/dh1tw/remoteAudio/audio/scReader"
	"github.com/dh1tw/remoteAudio/audio/scWriter"
	// "github.com/dh1tw/remoteAudio/audio/wavWriter"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	"github.com/gordonklaus/portaudio"
	"github.com/nats-io/go-nats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverMqttCmd represents the mqtt command
var natsServerCmd = &cobra.Command{
	Use:   "natsserver",
	Short: "nats server",
	Long:  `nats server`,
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

	opusBitrate := viper.GetInt("opus.bitrate")
	opusComplexity := viper.GetInt("opus.complexity")
	opusApplication, err := GetOpusApplication(viper.GetString("opus.application"))
	if err != nil {
		log.Fatal(err)
	}
	opusMaxBandwidth, err := GetOpusMaxBandwith(viper.GetString("opus.max-bandwidth"))
	if err != nil {
		log.Fatal(err)
	}

	natsUsername := viper.GetString("nats.username")
	natsPassword := viper.GetString("nats.password")
	natsBrokerURL := viper.GetString("nats.broker-url")
	natsBrokerPort := viper.GetInt("nats.broker-port")

	portaudio.Initialize()
	defer portaudio.Terminate()

	mic, err := scWriter.NewScWriter(
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

	radioAudio, err := scReader.NewScReader(
		scReader.DeviceName(iDeviceName),
		scReader.Channels(iChannels),
		scReader.Samplerate(iSamplerate),
		scReader.Latency(iLatency),
		scReader.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	fromNetwork, err := pbReader.NewPbReader()
	if err != nil {
		log.Fatal(err)
	}

	natsc, err := nats.Connect("nats://"+natsBrokerURL+":"+strconv.Itoa(natsBrokerPort),
		nats.UserInfo(natsUsername, natsPassword))
	if err != nil {
		log.Fatal(err)
	}

	// subscribe to the audio topic and enqueue the raw data into the pbReader
	natsc.Subscribe("toRadio", func(m *nats.Msg) {
		err := fromNetwork.Enqueue(m.Data)
		if err != nil {
			log.Println(err)
		}
	})

	// Callback which is called by pbWriter to push the audio
	// packets to the network
	toWireCb := func(data []byte) {
		err := natsc.Publish("fromRadio", data)
		if err != nil {
			log.Println(err)
		}
	}

	// opus Encoder for PbWriter
	opusEncoder, err := opus.NewEncoder(
		opus.Bitrate(opusBitrate),
		opus.Complexity(opusComplexity),
		opus.Channels(iChannels),
		opus.Samplerate(iSamplerate),
		opus.Application(opusApplication),
		opus.MaxBandwidth(opusMaxBandwidth),
	)

	toNetwork, err := pbWriter.NewPbWriter(toWireCb,
		pbWriter.Encoder(opusEncoder),
		pbWriter.Samplerate(iSamplerate),
		pbWriter.Channels(iChannels),
		pbWriter.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		log.Fatal(err)
	}

	rx, err := chain.NewChain(chain.DefaultSource("radioAudio"),
		chain.DefaultSink("toNetwork"))
	if err != nil {
		log.Fatal(err)
	}

	tx, err := chain.NewChain(chain.DefaultSource("fromNetwork"),
		chain.DefaultSink("mic"))
	if err != nil {
		log.Fatal(err)
	}

	tx.Sources.AddSource("fromNetwork", fromNetwork)
	tx.Sinks.AddSink("mic", mic, false)

	rx.Sources.AddSource("radioAudio", radioAudio)
	rx.Sinks.AddSink("toNetwork", toNetwork, false)

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	// set callback to process audio from the network
	rx.Sources.SetCb(rx.FromSourcesToSinksCb)
	// start streaming to the network immediately
	rx.Sinks.EnableSink("toNetwork", true)
	rx.Sources.SetSource("radioAudio")

	// set callback to process audio to be send to the radio
	tx.Sources.SetCb(tx.FromSourcesToSinksCb)
	// stream immediately audio from the network to the radio
	tx.Sources.SetSource("fromNetwork")
	tx.Sinks.EnableSink("mic", true)

	for {
		select {
		case sig := <-osSignals:
			if sig == os.Interrupt {
				// TBD: close also router (and all sinks)
				mic.Close()
				radioAudio.Close()
				return
			}
		}
	}
}

type natsServer struct {
	fromRadioSinks   audio.Router   //rx path
	fromRadioSources audio.Selector //rx path
	toRadioSinks     audio.Router   //tx path
	toRadioSources   audio.Selector //tx path
	isPlaying        bool
}

func (ns *natsServer) toRxSinksCb(data audio.Msg) {
	err := ns.fromRadioSinks.Write(data)
	if err != nil {
		// handle Error -> remove source
	}
	if data.EOF {
		// switch back to default source
		ns.fromRadioSinks.Flush()
		if err := ns.fromRadioSources.SetSource("toNetwork"); err != nil {
			log.Println(err)
		}
	}
}

func (ns *natsServer) toTxSinksCb(data audio.Msg) {
	err := ns.toRadioSinks.Write(data)
	if err != nil {
		// handle Error -> remove source
	}
	if data.EOF {
		// switch back to default source
		ns.toRadioSinks.Flush()
		if err := ns.toRadioSources.SetSource("fromNetwork"); err != nil {
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
