package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dh1tw/remoteAudio/webserver"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/transport"
	natsBroker "github.com/micro/go-plugins/broker/nats"
	natsReg "github.com/micro/go-plugins/registry/nats"
	"github.com/micro/go-plugins/selector/named"
	natsTr "github.com/micro/go-plugins/transport/nats"

	// _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/pbReader"
	"github.com/dh1tw/remoteAudio/audio/pbWriter"
	"github.com/dh1tw/remoteAudio/audio/scReader"
	"github.com/dh1tw/remoteAudio/audio/scWriter"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	"github.com/gordonklaus/portaudio"
	"github.com/nats-io/nats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverMqttCmd represents the mqtt command
var natsClientCmd = &cobra.Command{
	Use:   "natsclient",
	Short: "nats client",
	Long:  `nats client`,
	Run:   natsAudioClient,
}

func init() {
	serverCmd.AddCommand(natsClientCmd)
	natsClientCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	natsClientCmd.Flags().IntP("broker-port", "p", 4222, "Broker Port")
	natsClientCmd.Flags().StringP("password", "P", "", "NATS Password")
	natsClientCmd.Flags().StringP("username", "U", "", "NATS Username")
	natsClientCmd.Flags().StringP("radio", "Y", "myradio", "Radio ID")
	natsClientCmd.Flags().StringP("http-host", "w", "127.0.0.1", "Host (use '0.0.0.0' to listen on all network adapters)")
	natsClientCmd.Flags().StringP("http-port", "k", "9090", "Port to access the web interface")
	natsClientCmd.Flags().Int32("tx-volume", 70, "volume of tx audio stream on startup")
	natsClientCmd.Flags().Int32("rx-volume", 70, "volume of rx audio stream on startup")
	natsClientCmd.Flags().BoolP("stream-on-startup", "t", false, "start sending audio on startup")
}

func natsAudioClient(cmd *cobra.Command, args []string) {

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
	viper.BindPFlag("http.host", cmd.Flags().Lookup("http-host"))
	viper.BindPFlag("http.port", cmd.Flags().Lookup("http-port"))
	viper.BindPFlag("audio.rx-volume", cmd.Flags().Lookup("rx-volume"))
	viper.BindPFlag("audio.tx-volume", cmd.Flags().Lookup("tx-volume"))
	viper.BindPFlag("audio.stream-on-startup", cmd.Flags().Lookup("stream-on-startup"))

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

	rxVolume := viper.GetInt("audio.rx-volume")
	txVolume := viper.GetInt("audio.tx-volume")
	streamOnStartup := viper.GetBool("audio.stream-on-startup")

	natsUsername := viper.GetString("nats.username")
	natsPassword := viper.GetString("nats.password")
	natsBrokerURL := viper.GetString("nats.broker-url")
	natsBrokerPort := viper.GetInt("nats.broker-port")

	httpHost := viper.GetString("http.host")
	httpPort := viper.GetInt("http.port")
	natsAddr := fmt.Sprintf("nats://%s:%v", natsBrokerURL, natsBrokerPort)

	portaudio.Initialize()
	defer portaudio.Terminate()

	// start from default nats config and add the common options
	nopts := nats.GetDefaultOptions()
	nopts.Servers = []string{natsAddr}
	nopts.User = natsUsername
	nopts.Password = natsPassword

	disconnectedHdlr := func(conn *nats.Conn) {
		log.Println("connection to nats broker closed")
		// connClosed <- struct{}{}
	}

	errorHdlr := func(conn *nats.Conn, sub *nats.Subscription, err error) {
		log.Printf("Error Handler called (%s): %s", sub.Subject, err)
	}
	nopts.AsyncErrorCB = errorHdlr

	regNatsOpts := nopts
	brNatsOpts := nopts
	trNatsOpts := nopts
	regNatsOpts.DisconnectedCB = disconnectedHdlr
	regNatsOpts.Name = "remoteAudio.client:registry"
	brNatsOpts.Name = "remoteAudio.client:broker"
	trNatsOpts.Name = "remoteAudio.client:transport"

	regTimeout := registry.Timeout(time.Second * 2)
	trTimeout := transport.Timeout(time.Second * 2)

	reg := natsReg.NewRegistry(natsReg.Options(regNatsOpts), regTimeout)
	tr := natsTr.NewTransport(natsTr.Options(trNatsOpts), trTimeout)
	br := natsBroker.NewBroker(natsBroker.Options(brNatsOpts))
	cl := client.NewClient(
		client.Broker(br),
		client.Transport(tr),
		client.Registry(reg),
		client.PoolSize(1),
		client.PoolTTL(time.Hour*8760), // one year - don't TTL our connection
		client.Selector(named.NewSelector()),
		// client.Selector(cache.NewSelector(selector.Registry(reg))),
	)

	nc := natsClient{
		broker:       br,
		client:       cl,
		rxAudioTopic: fmt.Sprintf("shackbus.radio.%s.audio.rx", viper.GetString("nats.radio")),
		txAudioTopic: fmt.Sprintf("shackbus.radio.%s.audio.tx", viper.GetString("nats.radio")),
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
	speaker.SetVolume(float32(rxVolume) / 100)

	mic, err := scReader.NewScReader(
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

	// opus Encoder for PbWriter
	opusEncoder, err := opus.NewEncoder(
		opus.Bitrate(opusBitrate),
		opus.Complexity(opusComplexity),
		opus.Channels(iChannels),
		opus.Samplerate(iSamplerate),
		opus.Application(opusApplication),
		opus.MaxBandwidth(opusMaxBandwidth),
	)

	toNetwork, err := pbWriter.NewPbWriter(
		pbWriter.Encoder(opusEncoder),
		pbWriter.Samplerate(iSamplerate),
		pbWriter.Channels(iChannels),
		pbWriter.FramesPerBuffer(audioFramesPerBuffer),
		pbWriter.UserID(natsUsername),
		pbWriter.ToWireCb(nc.toWireCb),
	)

	if err != nil {
		log.Fatal(err)
	}
	toNetwork.SetVolume(float32(txVolume) / 100)

	rx, err := chain.NewChain(chain.DefaultSource("fromNetwork"),
		chain.DefaultSink("speaker"))
	if err != nil {
		log.Fatal(err)
	}

	tx, err := chain.NewChain(chain.DefaultSource("mic"),
		chain.DefaultSink("toNetwork"))
	if err != nil {
		log.Fatal(err)
	}

	rx.Sources.AddSource("fromNetwork", fromNetwork)
	rx.Sinks.AddSink("speaker", speaker, false)

	tx.Sources.AddSource("mic", mic)
	tx.Sinks.AddSink("toNetwork", toNetwork, false)

	nc.rx = rx
	nc.tx = tx
	nc.fromNetwork = fromNetwork

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	// start streaming to the network immediately
	rx.Sinks.EnableSink("speaker", true)
	rx.Sources.SetSource("fromNetwork")

	if streamOnStartup {
		tx.Sinks.EnableSink("toNetwork", true)
	}
	tx.Sources.SetSource("mic")

	remoteRxOn := true

	web, err := webserver.NewWebServer(httpHost, httpPort, remoteRxOn, rx, tx)
	if err != nil {
		log.Fatal(err)
	}

	if err := br.Connect(); err != nil {
		log.Fatal("broker:", err)
	}

	if err := cl.Init(); err != nil {
		log.Fatal(err)
		return
	}

	go web.Start()

	nc.initialized = true

	for {
		select {
		case sig := <-osSignals:
			if sig == os.Interrupt {
				// TBD: close also router (and all sinks)
				mic.Close()
				speaker.Close()
				return
			}
		}
	}
}

type natsClient struct {
	client       client.Client
	broker       broker.Broker
	rx           *chain.Chain
	tx           *chain.Chain
	fromNetwork  *pbReader.PbReader
	rxAudioTopic string
	txAudioTopic string
	initialized  bool
}

func (nc *natsClient) enqueueFromWire(pub broker.Publication) error {
	if nc.fromNetwork == nil {
		return nil
	}
	if !nc.initialized {
		return nil
	}
	return nc.fromNetwork.Enqueue(pub.Message().Body)
}

func (nc *natsClient) toWireCb(data []byte) {

	if !nc.initialized {
		return
	}

	// Callback which is called by pbWriter to push the audio
	// packets to the network
	msg := &broker.Message{
		Body: data,
	}
	err := nc.broker.Publish(nc.txAudioTopic, msg)
	if err != nil {
		log.Println(err)
	}
}

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
