package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/comms"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

type Hub struct {
	muClients    sync.Mutex
	clients      map[*Client]bool
	broadcast    chan []byte
	addClient    chan *Client
	removeClient chan *Client
	events       *pubsub.PubSub
	muAppState   sync.RWMutex
	appState     ApplicationState
}

var hub = Hub{
	broadcast:    make(chan []byte),
	addClient:    make(chan *Client),
	removeClient: make(chan *Client),
	muClients:    sync.Mutex{},
	clients:      make(map[*Client]bool),
	events:       nil,
	muAppState:   sync.RWMutex{},
	appState:     ApplicationState{},
}

type WebServerSettings struct {
	Events *pubsub.PubSub
}

type ApplicationState struct {
	ConnectionStatus *bool    `json:"connectionStatus, omitempty"`
	ServerOnline     *bool    `json:"serverOnline, omitempty"`
	ServerAudioOn    *bool    `json:"serverAudioOn, omitempty"`
	TxUser           *string  `json:"txUser, omitempty"`
	Tx               *bool    `json:"tx, omitempty"`
	Latency          *int64   `json:"latency, omitempty"`
	Volume           *float32 `json:"volume, omitempty"`
}

type ClientMessage struct {
	RequestServerAudioOn *bool    `json:"serverAudioOn, omitempty"`
	SetPtt               *bool    `json:"ptt, omitempty"`
	SetVolume            *float32 `json:"volume, omitempty"`
}

func (hub *Hub) sendMsg() {
	hub.muAppState.RLock()
	data, err := json.Marshal(hub.appState)
	if err != nil {
		log.Println(err)
	}
	hub.muAppState.RUnlock()
	hub.muClients.Lock()

	for client := range hub.clients {

		client.send <- data
	}
	hub.muClients.Unlock()
}

// Update
func sendLatency(latency int64) {

	msg := ApplicationState{}
	msg.Latency = &latency
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println(err)
	}

	for client := range hub.clients {
		client.send <- data
	}
}

func (hub *Hub) handleClientMsg(data []byte) {
	msg := ClientMessage{}
	err := json.Unmarshal(data, &msg)

	if err != nil {
		log.Println("Webserver: unable to unmarshal ClientMessage", string(data))
		return
	}

	if msg.SetPtt != nil {
		hub.events.Pub(*msg.SetPtt, events.RecordAudioOn)
		hub.sendMsg()
	}

	if msg.RequestServerAudioOn != nil {
		hub.events.Pub(*msg.RequestServerAudioOn, events.RequestServerAudioOn)
		hub.sendMsg()
	}

	if msg.SetVolume != nil {
		hub.events.Pub(float32(math.Pow(float64(*msg.SetVolume), 3)), events.SetVolume)
		hub.muAppState.Lock()
		hub.appState.Volume = msg.SetVolume
		hub.muAppState.Unlock()
		fmt.Println("sent new volume", *msg.SetVolume)
	}
}

func (hub *Hub) start() {

	connectionStatusCh := hub.events.Sub(events.MqttConnStatus)
	serverAudioOnCh := hub.events.Sub(events.ServerAudioOn)
	serverOnlineCh := hub.events.Sub(events.ServerOnline)
	txCh := hub.events.Sub(events.RecordAudioOn)
	txUserCh := hub.events.Sub(events.TxUser)
	pingCh := hub.events.Sub(events.Ping)

	hub.appState.Volume = func() *float32 { var vol float32 = 1.0; return &vol }()

	for {
		select {
		case ev := <-serverAudioOnCh:
			state := ev.(bool)
			hub.muAppState.Lock()
			hub.appState.ServerAudioOn = &state
			hub.muAppState.Unlock()
			hub.sendMsg()

		case ev := <-serverOnlineCh:
			state := ev.(bool)
			hub.muAppState.Lock()
			hub.appState.ServerOnline = &state
			hub.muAppState.Unlock()
			hub.sendMsg()

		case ev := <-txCh:
			state := ev.(bool)
			hub.muAppState.Lock()
			hub.appState.Tx = &state
			hub.muAppState.Unlock()
			hub.sendMsg()

		case ev := <-txUserCh:
			txUser := ev.(string)
			hub.muAppState.Lock()
			hub.appState.TxUser = &txUser
			hub.muAppState.Unlock()
			hub.sendMsg()

		case ev := <-pingCh:
			ping := ev.(int64) / 2000000 // milliseconds (one way latency)
			sendLatency(ping)

		case ev := <-connectionStatusCh:
			cs := ev.(int)
			hub.muAppState.Lock()
			if cs == comms.CONNECTED {
				hub.appState.ConnectionStatus = func() *bool { b := true; return &b }()
			} else {
				hub.appState.ConnectionStatus = func() *bool { b := false; return &b }()
			}
			hub.muAppState.Unlock()
			hub.sendMsg()

		case client := <-hub.addClient:
			log.Println("WebSocket connected")
			hub.muClients.Lock()
			hub.clients[client] = true
			hub.muClients.Unlock()
			hub.sendMsg() // should be send only to connecting client

		case client := <-hub.removeClient:
			log.Println("WebSocket disconnected")
			hub.muClients.Lock()
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.send)
			}
			hub.muClients.Unlock()

		}
	}
}

type Client struct {
	ws   *websocket.Conn
	send chan []byte
}

func (c *Client) write() {
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
			c.ws.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (c *Client) read() {
	defer func() {
		hub.removeClient <- c
		c.ws.Close()
	}()

	for {
		_, data, err := c.ws.ReadMessage()
		fmt.Println("got msg")
		if err != nil {
			break
		}
		hub.handleClientMsg(data)
	}
}

func wsPage(res http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(res, req, nil)

	if err != nil {
		http.NotFound(res, req)
		return
	}

	client := &Client{
		ws:   conn,
		send: make(chan []byte),
	}

	hub.addClient <- client

	go client.write()
	go client.read()
}

func homePage(res http.ResponseWriter, req *http.Request) {
	http.ServeFile(res, req, "html/index.html")
}

func noDirListing(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func Webserver(s WebServerSettings) {

	hub.events = s.Events

	go hub.start()

	http.Handle("/static/",
		noDirListing(http.FileServer(http.Dir("html/"))))

	http.HandleFunc("/ws", wsPage)
	http.HandleFunc("/", homePage)
	http.ListenAndServe(":8080", nil)
}
