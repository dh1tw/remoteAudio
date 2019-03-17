package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	rice "github.com/GeertJohan/go.rice"
	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/trx"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type WebServerSettings struct {
	Events  *pubsub.PubSub
	Address string
	Port    int
}

// wsClient contains a Websocket client
type wsClient struct {
	removeClient chan<- *wsClient
	ws           *websocket.Conn
	send         chan []byte
}

type ApplicationState struct {
	TxOn           bool                   `json:"tx_on"`
	RxVolume       int                    `json:"rx_volume"`
	TxVolume       int                    `json:"tx_volume"`
	Connected      bool                   `json:"connected"`
	AudioServers   map[string]AudioServer `json:"audio_servers"`
	SelectedServer string                 `json:"selected_server"`
}

type AudioServer struct {
	Name    string `json:"name"`
	On      bool   `json:"rx_on"`
	TxUser  string `json:"tx_user"`
	Latency int    `json:"latency"`
}

var upgrader = websocket.Upgrader{}

type WebServer struct {
	sync.RWMutex
	url            string
	port           int
	wsClients      map[*wsClient]bool
	addWsClient    chan *wsClient
	removeWsClient chan *wsClient
	trx            *trx.Trx
}
type AudioControlState struct {
	On *bool `json:"on"`
}

type AudioControlVolume struct {
	Volume *int `json:"volume"`
}

type AudioControlSelected struct {
	Selected *bool `json:"selected"`
}

func NewWebServer(url string, port int, trx *trx.Trx) (*WebServer, error) {

	web := &WebServer{
		url:            url,
		port:           port,
		wsClients:      make(map[*wsClient]bool),
		addWsClient:    make(chan *wsClient),
		removeWsClient: make(chan *wsClient),
		trx:            trx,
	}

	return web, nil
}

func (web *WebServer) Start() {

	web.trx.SetNotifyServerChangeCb(web.updateWsClients)

	box := rice.MustFindBox("../html")

	fileServer := http.FileServer(box.HTTPBox())

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/api/v1.0/rx/volume", web.rxVolumeHdlr)
	router.HandleFunc("/api/v1.0/tx/volume", web.txVolumeHdlr)
	router.HandleFunc("/api/v1.0/tx/state", web.txStateHdlr)
	router.HandleFunc("/api/v1.0/servers", web.serversHdlr).Methods("GET")
	router.HandleFunc("/api/v1.0/server/{server}", web.serverHdlr).Methods("GET")
	router.HandleFunc("/api/v1.0/server/{server}/selected", web.serverSelectedHdlr)
	router.HandleFunc("/api/v1.0/server/{server}/state", web.serverStateHdlr)
	router.HandleFunc("/ws", web.webSocketHdlr)
	router.PathPrefix("/").Handler(fileServer)

	serverURL := fmt.Sprintf("%s:%d", web.url, web.port)

	log.Println("Webserver listening on", serverURL)

	go func() {
		log.Fatal(http.ListenAndServe(serverURL, router))
	}()

	for {
		select {
		case wsClient := <-web.addWsClient:
			log.Println("WebSocket client connected from", wsClient.ws.RemoteAddr())
			web.Lock()
			web.wsClients[wsClient] = true
			web.Unlock()
			web.updateWsClients()

		case wsClient := <-web.removeWsClient:
			log.Println("WebSocket client disconnected", wsClient.ws.RemoteAddr())
			web.Lock()
			if _, ok := web.wsClients[wsClient]; ok {
				delete(web.wsClients, wsClient)
				close(wsClient.send)
			}
			web.Unlock()
		}
	}
}

func (web *WebServer) getAppState() (ApplicationState, error) {
	web.RLock()
	defer web.RUnlock()

	txState, err := web.trx.GetTxState()
	if err != nil {
		log.Println(err)
	}

	rxVolume, err := web.trx.GetRxVolume()
	if err != nil {
		log.Println(err)
	}

	txVolume, err := web.trx.GetTxVolume()
	if err != nil {
		log.Println(err)
	}

	asNames := web.trx.Servers()
	audioServers := make(map[string]AudioServer)

	for _, asName := range asNames {

		svr, exists := web.trx.Server(asName)
		if !exists {
			break
		}
		as := AudioServer{
			Name:    svr.Name(),
			On:      svr.RxOn(),
			TxUser:  svr.TxUser(),
			Latency: svr.Latency(),
		}

		audioServers[as.Name] = as
	}

	appState := ApplicationState{
		TxOn:           txState,
		RxVolume:       int(rxVolume * 100),
		TxVolume:       int(txVolume * 100),
		AudioServers:   audioServers,
		SelectedServer: web.trx.SelectedServer(),
	}

	return appState, nil
}

func (web *WebServer) updateWsClients() {

	appState, err := web.getAppState()
	if err != nil {
		log.Println("getAppState:", err)
		return
	}

	data, err := json.Marshal(appState)
	if err != nil {
		log.Println(err)
	}
	for client := range web.wsClients {
		client.send <- data
	}
}

func (c *wsClient) write() {
	defer func() {
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println(err)
			}
		}
	}
}

func (c *wsClient) read() {
	defer func() {
		c.removeClient <- c
		c.ws.Close()
	}()

	for {
		// ignore received messages
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
	}
}
