package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	natsBroker "github.com/asim/go-micro/plugins/broker/nats/v3"
	natsReg "github.com/asim/go-micro/plugins/registry/nats/v3"
	natsTr "github.com/asim/go-micro/plugins/transport/nats/v3"

	// _ "net/http/pprof"
	"github.com/asim/go-micro/v3/client"
	"github.com/asim/go-micro/v3/registry"
	"github.com/asim/go-micro/v3/transport"
	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/nodes/vox"
	"github.com/dh1tw/remoteAudio/audio/sinks/pbWriter"
	"github.com/dh1tw/remoteAudio/audio/sinks/scWriter"
	"github.com/dh1tw/remoteAudio/audio/sources/pbReader"
	"github.com/dh1tw/remoteAudio/audio/sources/scReader"
	"github.com/dh1tw/remoteAudio/audiocodec/opus"
	"github.com/dh1tw/remoteAudio/proxy"
	"github.com/dh1tw/remoteAudio/trx"
	"github.com/dh1tw/remoteAudio/webserver"
	"github.com/gordonklaus/portaudio"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serverMqttCmd represents the mqtt command
var natsClientCmd = &cobra.Command{
	Use:   "nats",
	Short: "NATS Client",
	Long: `NATS Client for bi-directional audio streaming

The audio streaming is done through the protocol NATS. You need a NATS
broker up and running to which the client can connect to.

In order to find the supported audio devices and audio host APIs
for your platform run:

$ remoteAudio(.exe) enumerate

It might be worth trying different host APIs since some provide lower latency
than others.

You can interact with the remoteAudio client through it's integrated
web server / REST API. In order to use the REST API check the project's
wiki https://github.com/dh1tw/remoteAudio/wiki.

The Web Interface / REST API provides you all available audio servers.
Multiple clients can listen to same audio server however only one can
send the audio at any time.
`,
	Run: natsAudioClient,
}

func init() {
	clientCmd.AddCommand(natsClientCmd)
	natsClientCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	natsClientCmd.Flags().IntP("broker-port", "p", 4222, "Broker Port")
	natsClientCmd.Flags().StringP("password", "P", "", "NATS Password")
	natsClientCmd.Flags().StringP("username", "U", "", "NATS Username")
	natsClientCmd.Flags().StringP("server-name", "Y", "", "default audio server (e.g. 'ts480')")
	natsClientCmd.Flags().StringP("http-host", "w", "127.0.0.1", "Host (use '0.0.0.0' to listen on all network adapters)")
	natsClientCmd.Flags().StringP("http-port", "k", "9090", "Port to access the web interface")
	natsClientCmd.Flags().Int32("tx-volume", 70, "volume of tx audio stream on startup")
	natsClientCmd.Flags().Int32("rx-volume", 70, "volume of rx audio stream on startup")
	natsClientCmd.Flags().BoolP("stream-on-startup", "t", false, "start the local and remote audio streams on startup")
	natsClientCmd.Flags().Bool("vox", false, "enable vox (voice activation)")
	natsClientCmd.Flags().Float32("vox-threshold", 0.1, "vox threshold (0...1)")
	natsClientCmd.Flags().Duration("vox-holdtime", time.Millisecond*500, "vox hold time")
}

func natsAudioClient(cmd *cobra.Command, args []string) {

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
		exit(err)
	}

	// bind the pflags to viper settings
	viper.BindPFlag("nats.broker-url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("nats.broker-port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("nats.password", cmd.Flags().Lookup("password"))
	viper.BindPFlag("nats.username", cmd.Flags().Lookup("username"))
	viper.BindPFlag("server.name", cmd.Flags().Lookup("server-name"))
	viper.BindPFlag("http.host", cmd.Flags().Lookup("http-host"))
	viper.BindPFlag("http.port", cmd.Flags().Lookup("http-port"))
	viper.BindPFlag("audio.rx-volume", cmd.Flags().Lookup("rx-volume"))
	viper.BindPFlag("audio.tx-volume", cmd.Flags().Lookup("tx-volume"))
	viper.BindPFlag("audio.stream-on-startup", cmd.Flags().Lookup("stream-on-startup"))
	viper.BindPFlag("audio.vox", cmd.Flags().Lookup("vox"))
	viper.BindPFlag("audio.vox-threshold", cmd.Flags().Lookup("vox-threshold"))
	viper.BindPFlag("audio.vox-holdtime", cmd.Flags().Lookup("vox-holdtime"))

	// profiling server
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant
	audioFramesPerBuffer := viper.GetInt("audio.frame-length")

	oDeviceName := viper.GetString("output-device.device-name")
	oHostAPI := viper.GetString("output-device.hostapi")
	oSamplerate := viper.GetFloat64("output-device.samplerate")
	oLatency := viper.GetDuration("output-device.latency")
	oChannels := viper.GetInt("output-device.channels")
	oRingBufferSize := viper.GetInt("audio.rx-buffer-length")

	iDeviceName := viper.GetString("input-device.device-name")
	iHostAPI := viper.GetString("input-device.hostapi")
	iSamplerate := viper.GetFloat64("input-device.samplerate")
	iLatency := viper.GetDuration("input-device.latency")
	iChannels := viper.GetInt("input-device.channels")

	opusBitrate := viper.GetInt("opus.bitrate")
	opusComplexity := viper.GetInt("opus.complexity")
	//values checked before

	opusApplication, err := getOpusApplication(viper.GetString("opus.application"))
	if err != nil {
		exit(err)
	}
	opusMaxBandwidth, err := getOpusMaxBandwith(viper.GetString("opus.max-bandwidth"))
	if err != nil {
		exit(err)
	}

	rxVolume := viper.GetInt("audio.rx-volume")
	txVolume := viper.GetInt("audio.tx-volume")
	streamOnStartup := viper.GetBool("audio.stream-on-startup")

	natsUsername := viper.GetString("nats.username")
	natsPassword := viper.GetString("nats.password")
	natsBrokerURL := viper.GetString("nats.broker-url")
	natsBrokerPort := viper.GetInt("nats.broker-port")
	serverName := viper.GetString("server.name")

	portaudio.Initialize()
	defer portaudio.Terminate()

	if len(serverName) > 0 && strings.ContainsAny(serverName, " _\n\r") {
		exit(fmt.Errorf("forbidden character in server name '%s'", serverName))
	}

	httpHost := viper.GetString("http.host")
	httpPort := viper.GetInt("http.port")
	natsAddr := fmt.Sprintf("nats://%s:%v", natsBrokerURL, natsBrokerPort)

	// start from default nats config and add the common options
	nopts := nats.GetDefaultOptions()
	nopts.Servers = []string{natsAddr}
	nopts.User = natsUsername
	nopts.Password = natsPassword

	disconnectedHdlr := func(conn *nats.Conn) {
		exit(fmt.Errorf("connection to nats broker closed"))
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

	regTimeout := registry.Timeout(time.Second * 3)
	trTimeout := transport.Timeout(time.Second * 3)

	reg := natsReg.NewRegistry(natsReg.Options(regNatsOpts), regTimeout)
	tr := natsTr.NewTransport(natsTr.Options(trNatsOpts), trTimeout)
	br := natsBroker.NewBroker(natsBroker.Options(brNatsOpts))
	cl := client.NewClient(
		client.Broker(br),
		client.Transport(tr),
		client.Registry(reg),
		client.PoolSize(1),
		client.PoolTTL(time.Hour*8760), // one year - don't TTL our connection
		client.ContentType("application/proto-rpc"),
	)

	speaker, err := scWriter.NewScWriter(
		scWriter.HostAPI(oHostAPI),
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
	speaker.SetVolume(float32(rxVolume) / 100)

	mic, err := scReader.NewScReader(
		scReader.HostAPI(iHostAPI),
		scReader.DeviceName(iDeviceName),
		scReader.Channels(iChannels),
		scReader.Samplerate(iSamplerate),
		scReader.Latency(iLatency),
		scReader.FramesPerBuffer(audioFramesPerBuffer),
	)
	if err != nil {
		exit(err)
	}

	fromNetwork, err := pbReader.NewPbReader()
	if err != nil {
		exit(err)
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
	if err != nil {
		exit(err)
	}

	toNetwork, err := pbWriter.NewPbWriter(
		pbWriter.Encoder(opusEncoder),
		pbWriter.Samplerate(iSamplerate),
		pbWriter.Channels(iChannels),
		pbWriter.FramesPerBuffer(audioFramesPerBuffer),
		pbWriter.UserID(natsUsername),
	)
	if err != nil {
		exit(err)
	}
	toNetwork.SetVolume(float32(txVolume) / 100)

	var _trx *trx.Trx

	voxStateChanged := func(newState bool) {
		if err := _trx.SetVOX(newState); err != nil {
			log.Printf("unable to set vox: %s\n", err.Error())
		}
	}
	voxEnabled := viper.GetBool("audio.vox")
	voxThreshold := viper.GetFloat64("audio.vox-threshold")
	voxHoldtime := viper.GetDuration("audio.vox-holdtime")

	_vox := vox.New(
		vox.Enabled(voxEnabled),
		vox.StateChanged(voxStateChanged),
		vox.Threshold(float32(voxThreshold)),
		vox.HoldTime(voxHoldtime))

	txChainOpts := []chain.Option{
		chain.DefaultSource("mic"),
		chain.Node(_vox),
		chain.DefaultSink("toNetwork"),
	}

	tx, err := chain.NewChain(txChainOpts...)
	if err != nil {
		exit(err)
	}

	rx, err := chain.NewChain(chain.DefaultSource("fromNetwork"),
		chain.DefaultSink("speaker"))
	if err != nil {
		exit(err)
	}

	// init the client
	if err := cl.Init(); err != nil {
		exit(err)
	}

	// connect to the broker
	if err := br.Connect(); err != nil {
		exit(err)
	}

	trxOpts := trx.Options{
		Rx:          rx,
		Tx:          tx,
		FromNetwork: fromNetwork,
		ToNetwork:   toNetwork,
		Broker:      br,
		Vox:         _vox,
	}

	_trx, err = trx.NewTrx(trxOpts)
	if err != nil {
		exit(err)
	}

	rx.Sources.AddSource("fromNetwork", fromNetwork)
	// set and enable speaker as default sink
	rx.Sinks.AddSink("speaker", speaker, true)
	// start streaming to the network immediately
	rx.Sources.SetSource("fromNetwork")

	tx.Sources.AddSource("mic", mic)
	tx.Sinks.AddSink("toNetwork", toNetwork, false)
	tx.Sources.SetSource("mic")

	// if a radio name is specified, create immediately
	// an audioServer object
	if len(serverName) > 0 {
		doneCh := make(chan struct{})
		audioSvr, err := proxy.NewAudioServer(serverName, cl, doneCh)
		if err != nil {
			exit(fmt.Errorf("audio server for %s unavailable", serverName))
		}

		_trx.AddServer(audioSvr)
		if err := _trx.SelectServer(serverName); err != nil {
			exit(err)
		}

		if streamOnStartup {
			if err := _trx.SetPTT(true); err != nil {
				exit(err)
			}
			if err := audioSvr.StartRxStream(); err != nil {
				exit(err)
			}
		}

		go func() {
			<-doneCh
			_trx.RemoveServer(serverName)
		}()
	}

	nc := natsClient{
		trx:    _trx,
		client: cl,
	}

	go nc.watchRegistry()

	web, err := webserver.NewWebServer(httpHost, httpPort, _trx)
	if err != nil {
		exit(err)
	}

	go web.Start()

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)
	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

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

// exit prints the error to stderr, stops portaudio and returns with exit
// code 1
func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	portaudio.Terminate()
	os.Exit(1)
}

type natsClient struct {
	client client.Client
	trx    *trx.Trx
}

// watchRegistry is a blocking function which continuously
// checks the registry for changes (new rotators being added/updated/removed).
func (nc *natsClient) watchRegistry() {
	watcher, err := nc.client.Options().Registry.Watch()
	if err != nil {
		exit(err)
	}

	for {
		res, err := watcher.Next()

		if err != nil {
			log.Println("watch error:", err)
		}

		if !isAudioServer(res.Service.Name) {
			continue
		}

		switch res.Action {

		case "create", "update":
			serverExists := nc.existsServer(res.Service.Name)

			if serverExists {
				continue
			}

			if err := nc.addServer(res.Service.Name); err != nil {
				// if we can not add the server, something is really wrong
				// so we better exit
				exit(err)
			}

		case "delete":
			nc.removeServer(res.Service.Name)
		}
	}
}

// isAudioServer checks a serviceName string if it is a shackbus audio Server
func isAudioServer(serviceName string) bool {

	if !strings.Contains(serviceName, "shackbus.radio.") {
		return false
	}
	return true
}

func (nc *natsClient) existsServer(aServerName string) bool {
	sName := nameFromFQSN(aServerName)
	_, exists := nc.trx.Server(sName)
	return exists
}

func (nc *natsClient) addServer(aServerName string) error {

	sName := nameFromFQSN(aServerName)

	doneCh := make(chan struct{})
	audioSvr, err := proxy.NewAudioServer(sName, nc.client, doneCh)
	if err != nil {
		return err
	}

	nc.trx.AddServer(audioSvr)

	go func() {
		<-doneCh
		nc.trx.RemoveServer(sName)
	}()

	// if this is the only server, then make it our default
	// and enable it
	if len(nc.trx.Servers()) == 1 {
		if err := nc.trx.SelectServer(sName); err != nil {
			return err
		}
	}

	return nil
}

func (nc *natsClient) removeServer(aServerName string) error {
	sName := nameFromFQSN(aServerName)
	return nc.trx.RemoveServer(sName)
}

//extract the service's name from its fully qualified service name (FQSN)
func nameFromFQSN(serviceName string) string {
	splitted := strings.Split(serviceName, ".")
	name := splitted[len(splitted)-2]
	return strings.Replace(name, "_", " ", -1)
}
