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
	RxOn        bool   `json:"rx_on"`
	TxOn        bool   `json:"tx_on"`
	RxVolume    int    `json:"rx_volume"`
	TxVolume    int    `json:"tx_volume"`
	TxUser      string `json:"tx_user"`
	Connected   bool   `json:"connected"`
	RadioOnline bool   `json:"radio_online"`
	Latency     int    `json:"latency"`
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

	box, err := rice.FindBox("../html")
	if err != nil {
		log.Fatal("webserver: box not found")
	}

	fileServer := http.FileServer(box.HTTPBox())

	router := mux.NewRouter().StrictSlash(true)
	// router.HandleFunc("/audio/rx/state", web.rxStateHdlr)
	router.HandleFunc("/api/rx/volume", web.rxVolumeHdlr)
	router.HandleFunc("/api/tx/state", web.txStateHdlr)
	router.HandleFunc("/api/tx/volume", web.txVolumeHdlr)
	router.HandleFunc("/ws", web.webSocketHdlr)
	router.HandleFunc("/", IndexHdlr)
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

func (web *WebServer) updateWsClients() {
	web.RLock()
	defer web.RUnlock()

	rxState, err := web.trx.GetRxState()
	if err != nil {
		log.Println(err)
	}
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

	txUser, err := web.trx.GetTxUser()
	if err != nil {
		log.Println(err)
	}

	appState := ApplicationState{
		RxOn:     rxState,
		TxOn:     txState,
		RxVolume: int(rxVolume * 100),
		TxVolume: int(txVolume * 100),
		TxUser:   txUser,
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
