package webserver

import (
	"encoding/json"
	"log"
	"net/http"
)

func IndexHdlr(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, "html/index.html")
}

func (web *WebServer) webSocketHdlr(w http.ResponseWriter, req *http.Request) {

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.NotFound(w, req)
		log.Printf("unable to open ws for %v\n", req.RemoteAddr)
		return
	}

	wsClient := &wsClient{
		ws:           conn,
		send:         make(chan []byte),
		removeClient: web.removeWsClient,
	}

	go wsClient.write()
	go wsClient.read()

	web.addWsClient <- wsClient
}

func (web *WebServer) txStateHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	_, active, err := web.Tx.Sinks.Sink("toNetwork")
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find protobuf serializer sink"))
		return
	}

	switch req.Method {
	case "GET":
		stateCtlMsg := &AudioControlState{
			On: &active,
		}
		if err := json.NewEncoder(w).Encode(stateCtlMsg); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to encode AudioControlState msg"))
		}

	case "PUT":
		var stateCtlMsg AudioControlState
		dec := json.NewDecoder(req.Body)

		if err := dec.Decode(&stateCtlMsg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid JSON"))
			return
		}
		if stateCtlMsg.On == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid Request"))
			return
		}
		if web.Tx.Sinks.EnableSink("toNetwork", *stateCtlMsg.On); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set tx state"))
		}
		web.Lock()
		web.appState.TxOn = *stateCtlMsg.On
		web.Unlock()
		web.updateWsClients()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) rxVolumeHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	speaker, _, err := web.Rx.Sinks.Sink("speaker")
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find speaker sink object"))
		return
	}

	switch req.Method {
	case "GET":
		vol := int(speaker.Volume() * 100)
		volCtlMsg := &AudioControlVolume{
			Volume: &vol,
		}
		if err := json.NewEncoder(w).Encode(volCtlMsg); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to encode AudioControlVolume msg"))
		}

	case "PUT":
		var volCtlMsg AudioControlVolume
		dec := json.NewDecoder(req.Body)

		if err := dec.Decode(&volCtlMsg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid JSON"))
			return
		}
		if volCtlMsg.Volume == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid Request"))
			return
		}
		if speaker.SetVolume(float32(*volCtlMsg.Volume) / 100); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set rx volume"))
		}
		web.Lock()
		web.appState.RxVolume = *volCtlMsg.Volume
		web.Unlock()
		web.updateWsClients()

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) txVolumeHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	toNetwork, _, err := web.Tx.Sinks.Sink("toNetwork")
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find protobuf serializer sink"))
		return
	}

	switch req.Method {

	case "GET":
		vol := int(toNetwork.Volume() * 100)
		volCtlMsg := &AudioControlVolume{
			Volume: &vol,
		}
		if err := json.NewEncoder(w).Encode(volCtlMsg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to encode AudioControlVolume msg"))
		}

	case "PUT":
		var volCtlMsg AudioControlVolume
		dec := json.NewDecoder(req.Body)
		if err := dec.Decode(&volCtlMsg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid JSON"))
			return
		}
		if volCtlMsg.Volume == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid Request"))
			return
		}
		if toNetwork.SetVolume(float32(*volCtlMsg.Volume) / 100); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set tx volume"))
		}
		web.Lock()
		web.appState.TxVolume = *volCtlMsg.Volume
		web.Unlock()
		web.updateWsClients()

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
