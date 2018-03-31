package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	// _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/pbReader"
	"github.com/dh1tw/remoteAudio/audio/pbWriter"
	"github.com/dh1tw/remoteAudio/audio/scReader"
	"github.com/dh1tw/remoteAudio/audio/scWriter"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gordonklaus/portaudio"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/server"
	natsBroker "github.com/micro/go-plugins/broker/nats"
	natsReg "github.com/micro/go-plugins/registry/nats"
	natsTr "github.com/micro/go-plugins/transport/nats"
	"github.com/nats-io/nats"
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
	natsServerCmd.Flags().BoolP("stream-on-startup", "t", false, "start streaming audio on startup")
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

	streamOnStartup := viper.GetBool("audio.stream-on-startup")

	natsUsername := viper.GetString("nats.username")
	natsPassword := viper.GetString("nats.password")
	natsBrokerURL := viper.GetString("nats.broker-url")
	natsBrokerPort := viper.GetInt("nats.broker-port")
	natsAddr := fmt.Sprintf("nats://%s:%v", natsBrokerURL, natsBrokerPort)

	portaudio.Initialize()
	defer portaudio.Terminate()

	// start from default nats config and add the common options
	nopts := nats.GetDefaultOptions()
	nopts.Servers = []string{natsAddr}
	nopts.User = natsUsername
	nopts.Password = natsPassword

	regNatsOpts := nopts
	brNatsOpts := nopts
	trNatsOpts := nopts

	serviceName := fmt.Sprintf("shackbus.radio.%s.audio", viper.GetString("nats.radio"))
	// we want to set the nats.Options.Name so that we can distinguish
	// them when monitoring the nats server with nats-top
	regNatsOpts.Name = serviceName + ":registry"
	brNatsOpts.Name = serviceName + ":broker"
	trNatsOpts.Name = serviceName + ":transport"

	// create instances of our nats Registry, Broker and Transport
	reg := natsReg.NewRegistry(natsReg.Options(regNatsOpts))
	br := natsBroker.NewBroker(natsBroker.Options(brNatsOpts))
	tr := natsTr.NewTransport(natsTr.Options(trNatsOpts))

	// this is a workaround since we must set server.Address with the
	// sanitized version of our service name. The server.Address will be
	// used in nats as the topic on which the server (transport) will be
	// listening on.
	svr := server.NewServer(
		server.Name(serviceName),
		server.Address(validateSubject(serviceName)),
		server.Transport(tr),
		server.Registry(reg),
		server.Broker(br),
	)

	// version is typically defined through a git tag and injected during
	// compilation; if not, just set it to "dev"
	if version == "" {
		version = "dev"
	}

	// let's create the new audio service
	rs := micro.NewService(
		micro.Name(serviceName),
		micro.RegisterInterval(time.Second*10),
		micro.Broker(br),
		micro.Transport(tr),
		micro.Registry(reg),
		micro.Version(version),
		micro.Server(svr),
	)

	ns := &natsServer{
		rxAudioTopic: fmt.Sprintf("%s.rx", strings.Replace(serviceName, " ", "_", -1)),
		txAudioTopic: fmt.Sprintf("%s.tx", strings.Replace(serviceName, " ", "_", -1)),
		service:      rs,
	}

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
		pbWriter.ToWireCb(ns.toWireCb),
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

	ns.rx = rx
	ns.tx = tx

	// initalize our service
	rs.Init()

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	if streamOnStartup {
		rx.Sinks.EnableSink("toNetwork", true)
	}
	rx.Sources.SetSource("radioAudio")

	// stream immediately audio from the network to the radio
	tx.Sources.SetSource("fromNetwork")
	tx.Sinks.EnableSink("mic", true)

	if err := br.Connect(); err != nil {
		log.Fatal("broker:", err)
	}

	// subscribe to the audio topic and enqueue the raw data into the pbReader
	sub, err := br.Subscribe(ns.txAudioTopic, ns.enqueueFromWire)
	if err != nil {
		log.Fatal("subscribe:", err)
	}

	sub.Topic() // can sub be removed?

	// register our Rotator RPC handler
	sbAudio.RegisterServerHandler(rs.Server(), ns)
	ns.initialized = true

	if err := rs.Run(); err != nil {
		log.Println(err)
		mic.Close()
		radioAudio.Close()
		// TBD: close also router (and all sinks)
		os.Exit(1)
	}
}

type natsServer struct {
	name         string
	service      micro.Service
	rx           *chain.Chain
	tx           *chain.Chain
	fromNetwork  *pbReader.PbReader
	rxAudioTopic string
	txAudioTopic string
	initialized  bool
}

func (ns *natsServer) enqueueFromWire(pub broker.Publication) error {
	if ns.fromNetwork == nil {
		return nil
	}
	if !ns.initialized {
		return nil
	}
	return ns.fromNetwork.Enqueue(pub.Message().Body)
}

func (ns *natsServer) toWireCb(data []byte) {

	if !ns.initialized {
		return
	}

	if ns.service == nil {
		return
	}

	if ns.service.Options().Broker == nil {
		return
	}

	// Callback which is called by pbWriter to push the audio
	// packets to the network
	msg := &broker.Message{
		Body: data,
	}

	err := ns.service.Options().Broker.Publish(ns.rxAudioTopic, msg)
	if err != nil {
		log.Println(err)
	}
}

func (ns *natsServer) GetCapabilities(ctx context.Context, in *sbAudio.None, out *sbAudio.Capabilities) error {
	out.Name = ns.name
	out.RxStreamAddress = ns.rxAudioTopic
	out.TxStreamAddress = ns.txAudioTopic
	return nil
}

func (ns *natsServer) StartStream(ctx context.Context, in, out *sbAudio.None) error {
	return ns.rx.Sinks.EnableSink("toNetwork", true)
}

func (ns *natsServer) StopStream(ctx context.Context, in, out *sbAudio.None) error {
	return ns.rx.Sinks.EnableSink("toNetwork", false)
}

func (ns *natsServer) Ping(ctx context.Context, in, out *sbAudio.PingPong) error {
	out = in
	return nil
}
