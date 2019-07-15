package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/dh1tw/remoteAudio/trx"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// wsClient contains a Websocket client
type wsClient struct {
	removeClient chan<- *wsClient
	ws           *websocket.Conn
	send         chan []byte
}

// ApplicationState is a data structure is provided through the
// /api/v{version}/servers endpoint.
type ApplicationState struct {
	TxOn           bool                   `json:"tx_on"`
	RxVolume       int                    `json:"rx_volume"`
	TxVolume       int                    `json:"tx_volume"`
	Connected      bool                   `json:"connected"`
	AudioServers   map[string]AudioServer `json:"audio_servers"`
	SelectedServer string                 `json:"selected_server"`
	VoxEnabled     bool                   `json:"vox_enabled"`
	VoxThreshold   float32                `json:"vox_threshold"`
	VoxHoldtime    time.Duration          `json:"vox_holdtime"`
}

// AudioServer is a data structure which is provided through the
// /api/v{version}/server/{radio} endpoint.
type AudioServer struct {
	Name    string `json:"name"`
	Index   int    `json:"index"`
	On      bool   `json:"rx_on"`
	TxUser  string `json:"tx_user"`
	Latency int    `json:"latency"`
}

var upgrader = websocket.Upgrader{}

// WebServer is the webserver's data structure holding internal
// variables.
type WebServer struct {
	sync.RWMutex
	url            string
	port           int
	router         *mux.Router
	apiVersion     string
	apiMatch       *regexp.Regexp
	wsClients      map[*wsClient]bool
	addWsClient    chan *wsClient
	removeWsClient chan *wsClient
	trx            *trx.Trx
}

// AudioControlState is a data structure which can be get/set through the
// /api/v{version}/server{radio}/state endpoint. It is used to start & stop
// the audio stream of a remote audio server.
type AudioControlState struct {
	On *bool `json:"on"`
}

// AudioControlVolume is a data structure which can be get/set through the
// /api/v{version}/rx/volume and /api/v{version}/tx/volume endpoints.
// It is used to adjust the local audio levels.
type AudioControlVolume struct {
	Volume *int `json:"volume"`
}

// AudioControlVox is a data structure which can be get/set through the
// /api/v{version}/tx/vox endpoint.
// It is used to get / set the local vox settings.
type AudioControlVox struct {
	VoxActive    *bool          `json:"vox_active"`
	VoxEnabled   *bool          `json:"vox_enabled"`
	VoxThreshold *float32       `json:"vox_threshold"`
	VoxHoldtime  *time.Duration `json:"vox_holdtime"`
}

// AudioControlSelected is a data structure which can be get/set through the
// /api/v{version}/server{radio}/selected endpoint to select a particular
// remote audio.
type AudioControlSelected struct {
	Selected *bool `json:"selected"`
}

// NewWebServer is the constructor method for a remoteAudio web server.
// The web server is only available on clients.
func NewWebServer(url string, port int, trx *trx.Trx) (*WebServer, error) {

	web := &WebServer{
		url:            url,
		port:           port,
		wsClients:      make(map[*wsClient]bool),
		addWsClient:    make(chan *wsClient),
		removeWsClient: make(chan *wsClient),
		apiVersion:     "1.0",
		apiMatch:       regexp.MustCompile(`api\/v\d\.\d\/`),
		router:         mux.NewRouter().StrictSlash(true),
		trx:            trx,
	}

	return web, nil
}

// Start will initialize the webserver and start serving on the designated
// interface & port.
func (web *WebServer) Start() {

	web.trx.SetNotifyServerChangeCb(web.updateWsClients)

	// load the HTTP routes with their respective endpoints
	web.routes()

	box := rice.MustFindBox("../html")

	fileServer := http.FileServer(box.HTTPBox())
	web.router.PathPrefix("/").Handler(fileServer)

	serverURL := fmt.Sprintf("%s:%d", web.url, web.port)

	log.Println("webserver listening on", serverURL)

	go func() {
		log.Fatal(http.ListenAndServe(serverURL, web.apiRedirectRouter(web.router)))
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

// getAppState returns the serialized information about the local configuration
// (e.g. volume levels) and all remote audio servers.
func (web *WebServer) getAppState() (ApplicationState, error) {
	web.RLock()
	defer web.RUnlock()

	txState, err := web.trx.TxState()
	if err != nil {
		log.Println(err)
	}

	rxVolume, err := web.trx.RxVolume()
	if err != nil {
		log.Println(err)
	}

	txVolume, err := web.trx.TxVolume()
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
			Index:   svr.Index(),
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
		VoxEnabled:     web.trx.VOXEnabled(),
		VoxHoldtime:    web.trx.VOXHoldTime(),
		VoxThreshold:   web.trx.VOXThreshold(),
	}

	return appState, nil
}

// updateWsClients sends the current state of the application to all
// connected websockets.
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

// write is a function which instantiates a go-routine through which
// concurrent writing to a websocket is handled.
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

// read is a function which instantiates a go-routine through which
// concurrent reading from a websocket is handled.
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
