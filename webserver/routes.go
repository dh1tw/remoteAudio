package webserver

func (web *WebServer) routes() {
	web.router.HandleFunc("/api/v1.0/rx/volume", web.rxVolumeHdlr)
	web.router.HandleFunc("/api/v1.0/tx/volume", web.txVolumeHdlr)
	web.router.HandleFunc("/api/v1.0/tx/state", web.txStateHdlr)
	web.router.HandleFunc("/api/v1.0/servers", web.serversHdlr).Methods("GET")
	web.router.HandleFunc("/api/v1.0/server/{server}", web.serverHdlr).Methods("GET")
	web.router.HandleFunc("/api/v1.0/server/{server}/selected", web.serverSelectedHdlr)
	web.router.HandleFunc("/api/v1.0/server/{server}/state", web.serverStateHdlr)
	web.router.HandleFunc("/ws", web.webSocketHdlr)
}
