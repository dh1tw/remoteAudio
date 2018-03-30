Vue.config.devtools = true

$.material.init();


var vm = new Vue({
    el: '#app',

    data: {
        ws: null, // Our websocket
        rxOn: false,
        txOn: false,
        txUser: null,
        connectionState: false,
        radioState: false,
        blockRxVolumeUpdate: false,
        blockTxVolumeUpdate: false,
        wsConnected: false,
        hideWsConnectionMsg: false,
    },
    mounted: function () {
        var self = this;
        this.ws = new ReconnectingWebSocket('ws://' + window.location.host + '/ws');
        this.ws.addEventListener('message', function (e) {
            var msg = JSON.parse(e.data);

            if (msg.rx_on !== null) {
                self.rxOn = msg.rx_on;
            }

            if (msg.tx_on !== null) {
                self.txOn = msg.tx_on;
            }

            if (msg.rx_volume !== null) {
                // only allow updates if we are not modifying the handle
                if (!self.blockRxVolumeUpdate) {
                    rxVolumeSlider.noUiSlider.set(msg.rx_volume);
                }
            }

            if (msg.tx_volume !== null) {
                if (!self.blockTxVolumeUpdate) {
                    txVolumeSlider.noUiSlider.set(msg.tx_volume);
                }
            }

            if (msg.tx_user !== null) {
                self.txUser = msg.tx_user;
            }

            if (msg.connected !== null) {
                self.connectionState = msg.connected;
            }

            if (msg.radio_online !== null) {
                self.radioState = msg.radio_online;
            }

            if (msg.latency !== null) {
                if (latencyChart.data.datasets[0].data.length >= 20) {
                    latencyChart.data.datasets[0].data.shift();
                }
                if (msg.ping > 500) {
                    latencyChart.data.datasets[0].data.push(500); // truncate
                } else {
                    latencyChart.data.datasets[0].data.push(msg.latency);
                }
                latencyChart.update(0.1);
            }
        });
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
    methods: {
        openWebsocket: function () {

        },
        sendRxOn: function () {
            this.$http.put("/api/rx/state",
                JSON.stringify({
                    on: !this.rxOn,
                }));
        },
        sendTxOn: function () {
            // if (this.serverAudioOn) {
            this.$http.put("/api/tx/state",
                JSON.stringify({
                    on: !this.txOn,
                }));
        },
        sendRxVolume: function (value) {
            this.$http.put("/api/rx/volume",
                JSON.stringify({
                    volume: Math.round(value)
                }));
        },
        sendTxVolume: function (value) {
            this.$http.put("/api/tx/volume",
                JSON.stringify({
                    volume: Math.round(value)
                }));
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
    //unblock the Volume slider updates through websocket
    txVolumeSlider.noUiSlider.on('end', function (values, handle) {
        vm.blockTxVolumeUpdate = false;
    });
});




var data = {
    labels: ["", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""],
    datasets: [{
        label: "Latency",
        fill: true,
        lineTension: 0.1,
        backgroundColor: "rgba(75,192,192,0.4)",
        borderColor: "rgba(75,192,192,1)",
        borderCapStyle: 'butt',
        borderDash: [],
        borderDashOffset: 0.0,
        borderJoinStyle: 'miter',
        pointBorderColor: "rgba(75,192,192,1)",
        pointBackgroundColor: "#fff",
        pointBorderWidth: 1,
        pointHoverRadius: 5,
        pointHoverBackgroundColor: "rgba(75,192,192,1)",
        pointHoverBorderColor: "rgba(220,220,220,1)",
        pointHoverBorderWidth: 2,
        pointRadius: 1,
        pointHitRadius: 10,
        data: [65],
        spanGaps: false,
    }]
};

var ctx = document.getElementById("latencyChart");
var latencyChart = new Chart(ctx, {
    type: 'line',
    data: data,
    options: {
        legend: {
            display: false,
        },
        // animation:{
        //     duration: 2000,
        //     animation: 'easeInOutQuad',
        // },
        responsive: true,
        layout: {
            padding: {
                left: 10,
                right: 20,
                top: 20
            },
        },
        scales: {
            yAxes: [{
                ticks: {
                    max: 500,
                    min: 0,
                    stepSize: 50
                }
            }],
        }
    }
});