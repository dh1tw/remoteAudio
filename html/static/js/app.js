$.material.init();



var vm = new Vue({
    el: '#app',

    data: {
        ws: null, // Our websocket
        tx: false,
        txUser: null,
        serverOnline: false,
        serverAudioOn: false,
        connectionStatus: false,
        connected: false,
        hideConnectionMsg: false,
        blockVolumeUpdate: false,
    },
    created: function () {
        var self = this;
        this.ws = new ReconnectingWebSocket('ws://' + window.location.host + '/ws');
        this.ws.addEventListener('message', function (e) {
            var msg = JSON.parse(e.data);
            if (msg.connectionStatus !== null) {
                self.connectionStatus = msg.connectionStatus;
            }
            if (msg.txUser !== null) {
                self.txUser = msg.txUser;
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
            if (msg.tx !== null) {
                self.tx = msg.tx;
            }
            if (msg.serverAudioOn !== null) {
                self.serverAudioOn = msg.serverAudioOn;
            }
            if (msg.serverOnline !== null) {
                self.serverOnline = msg.serverOnline;
            }
            if (msg.volume !== null){
                // only allow updates if we are not modifying the handle
                if (!self.blockVolumeUpdate){
                    volumeSlider.noUiSlider.set(msg.volume);
                }
            }
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
        sendVolume: function (value) {
            this.ws.send(
                JSON.stringify({
                    volume: value,
                })
            )
        }
    },
    // watch: {
    //     volume : function(val, oldVal){
    //         volumeSlider.set(val);
    //     }
    // }
});

var volumeSlider = document.getElementById('volumeSlider');

noUiSlider.create(volumeSlider, {
    start: [1],
    connect: [true, false],
    // tooltips: [ true ],
    range: {
        'min': 0,
        'max': 2
    },
    pips: { // Show a scale with the slider
        mode: 'steps',
        stepped: true,
        density: 5
    }
});

// block the Volume slider to be updated through websocket while we
// modify the slider
$(document).ready(function () {
    volumeSlider.noUiSlider.on('start', function (values, handle) {
        vm.blockVolumeUpdate = true;
    });
});

// send updates to server 
$(document).ready(function () {
    volumeSlider.noUiSlider.on('update', function (values, handle) {
        vm.sendVolume(Number(values[handle]));
    });
});

//unblock the Volume slider updates through websocket
$(document).ready(function () {
    volumeSlider.noUiSlider.on('end', function (values, handle) {
        vm.blockVolumeUpdate = false;
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


// socket.onopen = function (event) {
//     console.log("Socket opened successfully");
// }

// window.onbeforeunload = function (event) {
//     socket.close();
// }