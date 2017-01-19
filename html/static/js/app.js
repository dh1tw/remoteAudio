new Vue({
    el: '#app',

    data: {
        ws: null, // Our websocket
        tx: false,
        txUser: null,
        ping: null,
        serverOnline: false,
        serverAudioOn: false,
        connectionStatus: false,
        connected: false,
        hideConnectionMsg: false,
    },
    created: function () {
        var self = this;
        this.ws = new ReconnectingWebSocket('ws://' + window.location.host + '/ws');
        this.ws.addEventListener('message', function (e) {
            var msg = JSON.parse(e.data);
            self.connectionStatus = msg.connectionStatus;
            self.txUser = msg.txUser;
            self.ping = msg.ping;
            self.tx = msg.tx;
            self.serverAudioOn = msg.serverAudioOn;
            self.serverOnline = msg.serverOnline;
        });
        this.ws.addEventListener('open', function () {
            self.connected = true
            setTimeout(function () {
                self.hideConnectionMsg = true;
            }, 1500)
        });
        this.ws.addEventListener('close', function () {
            self.connected = false
            self.hideConnectionMsg = false;
        });

    },
    methods: {
        openWebsocket: function () {

        },
        sendRequestServerAudioOn: function () {
            this.ws.send(
                JSON.stringify({
                    serverAudioOn: !this.serverAudioOn,
                }));
        },
        sendPtt: function () {
            // if (this.serverAudioOn) {
                this.ws.send(
                    JSON.stringify({
                        ptt: !this.tx,
                    }));
            // }
        },
    }
})

// socket.onopen = function (event) {
//     console.log("Socket opened successfully");
// }

// window.onbeforeunload = function (event) {
//     socket.close();
// }