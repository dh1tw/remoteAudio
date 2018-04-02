var AudioServer = {
    template: `
                <div class="list-group-item">
                    <div class="row-action-primary vcenter">
                            <i class="fa fa-wifi" aria-hidden="true" v-bind:class="{'svr-selected': selected}" @click="setAudioServer"></i>
                    </div>
                    <div class="row-content">
                      <h4 class="list-group-item-heading svr-name">{{name}}</h4>
                      <button class="btn btn-default btn-raised" v-bind:class="{'btn-success': rxOn}" @click="setRxState"><i class="fa fa-volume-up" aria-hidden="true"></i> RX Audio</button>
                      <p>Latency: {{latency}} ms</p>
                    </div>
                </div>`,
    props: {
        name: String,
        rxOn: Boolean,
        txUser: String,
        latency: Number,
        selected: Boolean,
    },
    mounted: function () {},
    beforeDestroy: function () {},
    methods: {
        setAudioServer: function () {
            this.$emit('set-audioserver', this.name);
        },
        setRxState: function () {
            this.$emit('set-rxstate', this.name, !this.rxOn);
        },
    },
    computed: {},
    watch: {},
}