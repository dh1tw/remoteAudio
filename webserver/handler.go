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

func (web *WebServer) rxStateHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	state, err := web.trx.GetRxState()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find protobuf serializer sink"))
		return
	}

	switch req.Method {
	case "GET":
		stateCtlMsg := &AudioControlState{
			On: &state,
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
		if err := web.trx.SetRxState(*stateCtlMsg.On); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set tx state"))
		}
		web.updateWsClients()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) txStateHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	state, err := web.trx.GetTxState()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find protobuf serializer sink"))
		return
	}

	switch req.Method {
	case "GET":
		stateCtlMsg := &AudioControlState{
			On: &state,
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
		if err := web.trx.SetTxState(*stateCtlMsg.On); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set tx state"))
		}
		web.updateWsClients()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) rxVolumeHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	volume, err := web.trx.GetRxVolume()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find speaker sink object"))
		return
	}

	switch req.Method {
	case "GET":
		vol := int(volume * 100)
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
		if web.trx.SetRxVolume(float32(*volCtlMsg.Volume) / 100); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set rx volume"))
		}
		web.updateWsClients()

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) txVolumeHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	volume, err := web.trx.GetTxVolume()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to find protobuf serializer sink"))
		return
	}

	switch req.Method {

	case "GET":
		vol := int(volume * 100)
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
		if err := web.trx.SetTxVolume(float32(*volCtlMsg.Volume) / 100); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to set tx volume"))
		}
		web.updateWsClients()

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
