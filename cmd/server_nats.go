package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time" // _ "net/http/pprof"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/pbReader"
	"github.com/dh1tw/remoteAudio/audio/pbWriter"
	"github.com/dh1tw/remoteAudio/audio/scReader"
	"github.com/dh1tw/remoteAudio/audio/scWriter"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
	"github.com/gordonklaus/portaudio"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/server"
	natsBroker "github.com/micro/go-plugins/broker/nats"
	natsReg "github.com/micro/go-plugins/registry/nats"
	natsTr "github.com/micro/go-plugins/transport/nats"
	"github.com/nats-io/go-nats"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverMqttCmd represents the mqtt command
var natsServerCmd = &cobra.Command{
	Use:   "nats",
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
	natsServerCmd.Flags().StringP("radio", "Y", "", "radio name to which this audio server belongs (e.g. 'ts480'")
}

func natsAudioServer(cmd *cobra.Command, args []string) {

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if strings.Contains(err.Error(), "Not Found in") {
			fmt.Println("no config file found")
		} else {
			fmt.Fprintf(os.Stderr, "Error parsing config file %v: %v\n",
				viper.ConfigFileUsed(), err)
			os.Exit(1)
		}
	}

	// check if values from config file / pflags are valid
	if err := checkAudioParameterValues(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	// value checked before
	opusApplication, _ := getOpusApplication(viper.GetString("opus.application"))
	opusMaxBandwidth, _ := getOpusMaxBandwith(viper.GetString("opus.max-bandwidth"))

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

	radioName := viper.GetString("nats.radio")

	if len(radioName) == 0 {
		exit(fmt.Errorf("radio name missing"))
	}

	if strings.ContainsAny(radioName, " _\n\r") {
		exit(fmt.Errorf("forbidden character in radio name '%s'", radioName))
	}

	serviceName := fmt.Sprintf("shackbus.radio.%s.audio", radioName)

	// we want to set the nats.Options.Name so that we can distinguish
	// them when monitoring the nats server with nats-top
	regNatsOpts.Name = serviceName + ":registry"
	brNatsOpts.Name = serviceName + ":broker"
	trNatsOpts.Name = serviceName + ":transport"

	regTimeout := registry.Timeout(time.Second * 2)

	// create instances of our nats Registry, Broker and Transport
	reg := natsReg.NewRegistry(natsReg.Options(regNatsOpts), regTimeout)
	br := natsBroker.NewBroker(natsBroker.Options(brNatsOpts))
	tr := natsTr.NewTransport(natsTr.Options(trNatsOpts))

	// this is a workaround since we must set server.Address with the
	// sanitized version of our service name. The server.Address will be
	// used in nats as the topic on which the server (transport) will be
	// listening on.
	svr := server.NewServer(
		server.Name(serviceName),
		server.Address(validateSubject(serviceName)),
		server.RegisterInterval(time.Second*10),
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
		micro.Broker(br),
		micro.Transport(tr),
		micro.Registry(reg),
		micro.Version(version),
		micro.Server(svr),
	)

	// natsServer is a convenience object which contains all the long
	// living variable & objects of this application
	ns := &natsServer{
		rxAudioTopic: serviceName + ".rx",
		txAudioTopic: serviceName + ".tx",
		stateTopic:   serviceName + ".state",
		service:      rs,
		broker:       br,
	}

	// create an sound card writer (typically feeding audio into the
	// microphone of the transceiver)
	mic, err := scWriter.NewScWriter(
		scWriter.DeviceName(oDeviceName),
		scWriter.Channels(oChannels),
		scWriter.Samplerate(oSamplerate),
		scWriter.Latency(oLatency),
		scWriter.RingBufferSize(oRingBufferSize),
		scWriter.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		exit(err)
	}

	// create a soundcard reader (typically connected to the speaker
	// of the transceiver)
	radioAudio, err := scReader.NewScReader(
		scReader.DeviceName(iDeviceName),
		scReader.Channels(iChannels),
		scReader.Samplerate(iSamplerate),
		scReader.Latency(iLatency),
		scReader.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		exit(err)
	}

	// create a Protobuf reader through which will decode the incomming
	// data from the network
	fromNetwork, err := pbReader.NewPbReader()
	if err != nil {
		exit(err)
	}

	// opus Encoder for the protobuf writer
	opusEncoder, err := opus.NewEncoder(
		opus.Bitrate(opusBitrate),
		opus.Complexity(opusComplexity),
		opus.Channels(iChannels),
		opus.Samplerate(iSamplerate),
		opus.Application(opusApplication),
		opus.MaxBandwidth(opusMaxBandwidth),
	)
	if err != nil {
		exit(err)
	}

	// create a protobuf serializer which will encode our audio data
	// and send it on the wire
	toNetwork, err := pbWriter.NewPbWriter(
		pbWriter.Encoder(opusEncoder),
		pbWriter.Samplerate(iSamplerate),
		pbWriter.Channels(iChannels),
		pbWriter.FramesPerBuffer(audioFramesPerBuffer),
		pbWriter.ToWireCb(ns.toWireCb),
	)
	if err != nil {
		exit(err)
	}

	// create the receiving audio chain (from speaker to network)
	rx, err := chain.NewChain(chain.DefaultSource("radioAudio"),
		chain.DefaultSink("toNetwork"))
	if err != nil {
		exit(err)
	}

	// create the sending chain (from network to microphone)
	tx, err := chain.NewChain(chain.DefaultSource("fromNetwork"),
		chain.DefaultSink("mic"))
	if err != nil {
		exit(err)
	}

	// add audio sinks & sources to the tx audio chain
	tx.Sources.AddSource("fromNetwork", fromNetwork)
	tx.Sinks.AddSink("mic", mic, false)

	// add audio sinks & sources to the rx audio chain
	rx.Sources.AddSource("radioAudio", radioAudio)
	rx.Sinks.AddSink("toNetwork", toNetwork, false)

	// assign the rx and tx audio chain to our natsServer
	ns.rx = rx
	ns.tx = tx
	ns.fromNetwork = fromNetwork

	// initalize our micro service
	rs.Init()

	// before we annouce this service, we have to ensure that no other
	// service with the same name exists. Therefore we query the
	// registry for all other existing services.
	services, err := reg.ListServices()
	if err != nil {
		log.Fatal(err)
	}

	// if a service with this name already exists, then exit
	for _, service := range services {
		if service.Name == serviceName {
			exit(fmt.Errorf("service %s already exists", service.Name))
		}
	}

	// connect the broker
	if err := br.Connect(); err != nil {
		exit(fmt.Errorf("broker: %v", err))
	}

	// subscribe to the audio topic and enqueue the raw data into the pbReader
	sub, err := br.Subscribe(ns.txAudioTopic, ns.enqueueFromWire)
	if err != nil {
		exit(fmt.Errorf("subscribe: %v", err))
	}
	ns.txAudioSub = sub

	// register our Rotator RPC handler
	sbAudio.RegisterServerHandler(rs.Server(), ns)

	rx.Sources.SetSource("radioAudio")

	// stream immediately audio from the network to the radio
	tx.Sources.SetSource("fromNetwork")
	if err := tx.StartTx(); err != nil {
		exit(err)
	}

	// run the micro service
	if err := rs.Run(); err != nil {
		log.Println(err)
		mic.Close()
		radioAudio.Close()
		rx.Sources.Close()
		rx.Sinks.Close()
		tx.Sources.Close()
		tx.Sinks.Close()
	}
}

type natsServer struct {
	sync.RWMutex
	name         string
	service      micro.Service
	broker       broker.Broker
	rx           *chain.Chain
	tx           *chain.Chain
	fromNetwork  *pbReader.PbReader
	rxAudioTopic string
	txAudioTopic string
	txAudioSub   broker.Subscriber
	stateTopic   string
	rxOn         bool
	txUser       string
}

func (ns *natsServer) enqueueFromWire(pub broker.Publication) error {
	if ns.fromNetwork == nil {
		return nil
	}
	return ns.fromNetwork.Enqueue(pub.Message().Body)
}

// Callback which is called by pbWriter to push the audio
// packets to the network
func (ns *natsServer) toWireCb(data []byte) {

	if ns.broker == nil {
		log.Println("sendState: broker not set") // better Fatal?
	}

	msg := &broker.Message{
		Body: data,
	}

	err := ns.broker.Publish(ns.rxAudioTopic, msg)
	if err != nil {
		log.Println(err) // better fatal?
	}
}

func (ns *natsServer) sendState() error {
	ns.RLock()
	defer ns.RUnlock()

	if ns.broker == nil {
		return fmt.Errorf("sendState: broker not set")
	}

	state := sbAudio.State{
		RxOn:   ns.rxOn,
		TxUser: ns.txUser,
	}

	data, err := proto.Marshal(&state)
	if err != nil {
		return err
	}

	msg := &broker.Message{
		Body: data,
	}

	return ns.broker.Publish(ns.stateTopic, msg)
}

func (ns *natsServer) GetCapabilities(ctx context.Context, in *sbAudio.None, out *sbAudio.Capabilities) error {
	ns.RLock()
	defer ns.RUnlock()
	out.Name = ns.name
	out.RxStreamAddress = ns.rxAudioTopic
	out.TxStreamAddress = ns.txAudioTopic
	out.StateUpdatesAddress = ns.stateTopic
	return nil
}

func (ns *natsServer) GetState(ctx context.Context, in *sbAudio.None, out *sbAudio.State) error {
	rxOn, txUser, err := ns.getState()
	if err != nil {
		return err
	}
	out.RxOn = rxOn
	out.TxUser = txUser
	return nil
}

func (ns *natsServer) StartStream(ctx context.Context, in, out *sbAudio.None) error {

	if err := ns.rx.StartTx(); err != nil {
		log.Println("StartStream:", err)
		return err
	}

	ns.Lock()
	ns.rxOn = true
	ns.Unlock()

	if err := ns.sendState(); err != nil {
		log.Println("StartStream:", err)
		return err
	}
	return nil
}

func (ns *natsServer) StopStream(ctx context.Context, in, out *sbAudio.None) error {

	if err := ns.rx.StopTx(); err != nil {
		log.Println("StopStream:", err)
		return err
	}

	ns.Lock()
	ns.rxOn = false
	ns.Unlock()

	if err := ns.sendState(); err != nil {
		log.Println("StopStream:", err)
		return err
	}
	return nil
}

func (ns *natsServer) Ping(ctx context.Context, in, out *sbAudio.PingPong) error {
	out.Ping = in.Ping
	return nil
}

func (ns *natsServer) getState() (bool, string, error) {
	ns.RLock()
	defer ns.RUnlock()
	_, rxOn, err := ns.rx.Sinks.Sink("toNetwork")
	if err != nil {
		return false, "", err
	}
	return rxOn, ns.txUser, nil
}
