package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

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

	state, err := web.trx.TxState()
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
		if err := web.trx.SetPTT(*stateCtlMsg.On); err != nil {
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

	volume, err := web.trx.RxVolume()
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

	volume, err := web.trx.TxVolume()
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

func (web *WebServer) serverSelectedHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(req)
	asName := vars["server"]

	_, ok := web.trx.Server(asName)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - unable to find server %s", asName)))
		return
	}

	switch req.Method {
	case "GET":
		selected := false
		if web.trx.SelectedServer() == asName {
			selected = true
		}
		activeCtlMsg := &AudioControlSelected{
			Selected: &selected,
		}
		if err := json.NewEncoder(w).Encode(activeCtlMsg); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - unable to encode AudioControlActive msg"))
		}

	case "PUT":
		var selectedCtlMsg AudioControlSelected
		dec := json.NewDecoder(req.Body)

		if err := dec.Decode(&selectedCtlMsg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid JSON"))
			return
		}
		if selectedCtlMsg.Selected == nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("400 - invalid Request"))
			return
		}
		if err := web.trx.SelectServer(asName); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("500 - unable to select audio server %s", asName)))
		}
		web.updateWsClients()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) serverStateHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(req)
	asName := vars["server"]

	as, ok := web.trx.Server(asName)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - unable to find server %s", asName)))
		return
	}

	switch req.Method {
	case "GET":
		on := as.RxOn()
		stateCtlMsg := &AudioControlState{
			On: &on,
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
		var err error
		if *stateCtlMsg.On {
			err = as.StartRxStream()
		} else {
			err = as.StopRxStream()
		}
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(
				fmt.Sprintf("500 - unable to change audio server %s state to %v", asName, *stateCtlMsg.On)))
		}
		web.updateWsClients()
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (web *WebServer) serverHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(req)
	asName := vars["server"]

	as, ok := web.trx.Server(asName)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - unable to find server %s", asName)))
		return
	}

	serverMsg := &AudioServer{
		Name:    as.Name(),
		Index:   as.Index(),
		TxUser:  as.TxUser(),
		On:      as.RxOn(),
		Latency: as.Latency(),
	}
	if err := json.NewEncoder(w).Encode(serverMsg); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to encode AudioControlState msg"))
	}
}

func (web *WebServer) serversHdlr(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	appState, err := web.getAppState()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to execute query"))
		return
	}

	if err := json.NewEncoder(w).Encode(appState); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - unable to encode AudioControlState msg"))
	}
}
