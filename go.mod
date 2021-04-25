module github.com/dh1tw/remoteAudio

go 1.16

require (
	github.com/asim/go-micro/plugins/broker/nats/v3 v3.0.0-20210416163442-a91d1f7a3dbb
	github.com/asim/go-micro/plugins/registry/nats/v3 v3.0.0-20210416163442-a91d1f7a3dbb
	github.com/asim/go-micro/plugins/transport/nats/v3 v3.0.0-20210416163442-a91d1f7a3dbb
	github.com/asim/go-micro/v3 v3.5.1
	github.com/chewxy/math32 v1.0.6
	github.com/dh1tw/golang-ring v0.0.0-20180327112950-d11a99b5aede
	github.com/dh1tw/gosamplerate v0.1.2
	github.com/dh1tw/nolistfs v0.1.0
	github.com/go-audio/audio v1.0.0
	github.com/go-audio/wav v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/gordonklaus/portaudio v0.0.0-20200911161147-bb74aa485641
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/nats-io/nats.go v1.10.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	google.golang.org/protobuf v1.26.0
	gopkg.in/hraban/opus.v2 v2.0.0-20210415224706-ab1467d63813
)
