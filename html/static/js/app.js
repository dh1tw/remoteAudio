Vue.config.devtools = true

$.material.init();

var vm = new Vue({
    el: '#app',

    data: {
        ws: null, // Our websocket
        txOn: false,
        connectionState: false,
        blockRxVolumeUpdate: false,
        blockTxVolumeUpdate: false,
        audioServers: {},
        wsConnected: false,
        hideWsConnectionMsg: false,
    },
    components: {
        'audioservers': AudioServers,
    },
    mounted: function () {
        this.openWebsocket();
    },
    methods: {
        openWebsocket: function () {
            var self = this;
            this.ws = new ReconnectingWebSocket('ws://' + window.location.host + '/ws');
            this.ws.addEventListener('message', this.processWsMsg);
            this.ws.addEventListener('open', function () {
                self.wsConnected = true
                setTimeout(function () {
                    self.hideWsConnectionMsg = true;
                }, 1500)
            });
            this.ws.addEventListener('close', function () {
                self.wsConnected = false
                self.hideWsConnectionMsg = false;
            });
        },
        processWsMsg: function (rawMsg) {
            var msg = JSON.parse(rawMsg.data);
            // console.log(msg);

            if (msg.tx_on !== null) {
                this.txOn = msg.tx_on;
            }

            if (msg.rx_volume !== null) {
                // only allow updates if we are not modifying the handle
                if (!this.blockRxVolumeUpdate) {
                    if (rxVolumeSlider.noUiSlider.get() != msg.rx_volume) {
                        rxVolumeSlider.noUiSlider.set(msg.rx_volume);
                    }
                }
            }

            if (msg.tx_volume !== null) {
                if (!this.blockTxVolumeUpdate) {
                    if (txVolumeSlider.noUiSlider.get() != msg.tx_volume) {
                        txVolumeSlider.noUiSlider.set(msg.tx_volume);
                    }
                }
            }

            if (msg.connected !== null) {
                this.connectionState = msg.connected;
            }

            if (msg.audio_servers !== null) {
                this.updateAudioServers(msg.audio_servers)
            }

            if (msg.selected_server !== null) {
                this.selectServer(msg.selected_server);
            }
        },
        selectServer: function (asName) {

            if (asName == "") {
                return
            }
            if (!(asName in this.audioServers)) {
                return
            }

            self = this;
            Object.keys(this.audioServers).forEach(function(asName){
                self.$set(self.audioServers[asName], "selected", false);
            }),

            this.$set(this.audioServers[asName], "selected", true);
        },
        setAudioServer: function (audioServerName) {
            this.$http.put("/api/v1.0/server/" + audioServerName + "/selected",
                JSON.stringify({
                    selected: true,
                }));
        },
        setRxState: function (audioServerName, rxState) {
            this.$http.put("/api/v1.0/server/" + audioServerName + "/state",
                JSON.stringify({
                    on: rxState,
                }));
        },
        sendTxOn: function () {
            this.$http.put("/api/v1.0/tx/state",
                JSON.stringify({
                    on: !this.txOn,
                }));
        },
        sendRxVolume: function (value) {
            this.$http.put("/api/v1.0/rx/volume",
                JSON.stringify({
                    volume: Math.round(value)
                }));
        },
        sendTxVolume: function (value) {
            this.$http.put("/api/v1.0/tx/volume",
                JSON.stringify({
                    volume: Math.round(value)
                }));
        },
        updateAudioServers: function (aServers) {

            // copy this into the variable self, as this will change
            // within the forEach loops
            self = this;

            // iterate over audioServer objects and remove the ones which
            // are not included anymore
            Object.keys(this.audioServers).forEach(function (asName) {
                if (!(asName in aServers)) {
                    self.$delete(self.audioServers, asName)
                }
            })

            // iterate over the received audioServer objects. Create the object
            // or update the fields of the stored objects if necessary
            Object.keys(aServers).forEach(function (asName) {
                // if audioServer doesn't exist, add it
                if (!(asName in self.audioServers)) {
                    self.$set(self.audioServers, asName, aServers[asName])
                } else { //just update the fields
                    if (self.audioServers[asName].rx_on != aServers[asName].rx_on) {
                        self.audioServers[asName].rx_on = aServers[asName].rx_on
                    }
                    if (self.audioServers[asName].tx_user != aServers[asName].tx_user) {
                        self.audioServers[asName].tx_user = aServers[asName].tx_user
                    }
                    if (self.audioServers[asName].latency != aServers[asName].latency) {
                        self.audioServers[asName].latency = aServers[asName].latency
                    }
                }
            })
        },
        sortByKey: function(array, key) {
            return array.sort(function(a, b) {
                var x = a[key]; var y = b[key];
                return ((x < y) ? -1 : ((x > y) ? 1 : 0));
            });
        },
    },
    computed: {
        sortedAudioServers: function (){
            var servers = []
            for (svr in this.audioServers) {
                servers.push(this.audioServers[svr])
            }
            this.sortByKey(servers, "index")
            return servers
        },
    },
});

var rxVolumeSlider = document.getElementById('rxVolumeSlider');
var txVolumeSlider = document.getElementById('txVolumeSlider');

noUiSlider.create(rxVolumeSlider, {
    start: [1],
    connect: [true, false],
    range: {
        'min': 0,
        'max': 100,
    },
    pips: { // Show a scale with the slider
        mode: 'steps',
        stepped: true,
        density: 5
    }
});

noUiSlider.create(txVolumeSlider, {
    start: [1],
    connect: [true, false],
    range: {
        'min': 0,
        'max': 100,
    },
    pips: { // Show a scale with the slider
        mode: 'steps',
        stepped: true,
        density: 5
    }
});

// rxVolumeSlider
// block the Volume slider to be updated through websocket while we
// modify the slider
$(document).ready(function () {
    rxVolumeSlider.noUiSlider.on('start', function (values, handle) {
        vm.blockRxVolumeUpdate = true;
    });
    // send updates to server
    rxVolumeSlider.noUiSlider.on('update', function (values, handle) {
        if (vm.blockRxVolumeUpdate) {
            vm.sendRxVolume(Number(values[handle]));
        }
    });
    rxVolumeSlider.noUiSlider.on('change', function (values, handle) {
        vm.sendRxVolume(Number(values[handle]));
    });
    //unblock the Volume slider updates through websocket
    rxVolumeSlider.noUiSlider.on('end', function (values, handle) {
        vm.blockRxVolumeUpdate = false;
    });
});

// txVolumeSlider
$(document).ready(function () {
    txVolumeSlider.noUiSlider.on('start', function (values, handle) {
        vm.blockTxVolumeUpdate = true;
    });
    // send updates to server
    txVolumeSlider.noUiSlider.on('update', function (values, handle) {
        if (vm.blockTxVolumeUpdate) {
            vm.sendTxVolume(Number(values[handle]));
        }
    });
    txVolumeSlider.noUiSlider.on('change', function (values, handle) {
        vm.sendTxVolume(Number(values[handle]));
    });
    //unblock the Volume slider updates through websocket
    txVolumeSlider.noUiSlider.on('end', function (values, handle) {
        vm.blockTxVolumeUpdate = false;
    });
});