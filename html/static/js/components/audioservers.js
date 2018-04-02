var AudioServers = {
  template: `<div class="col-lg-4 col-md-4 col-sm-6">
                <div class="panel panel-primary">
                  <div class="panel-heading">
                    Audio Servers
                  </div>
                <div class="panel-body">
                  <div class="list-group">
                    <div v-for="server in servers" :key="servers">
                      <audioserver v-on:set-audioserver="setAudioServer"
                        v-on:set-rxstate="setRxState"
                        :selected=server.selected
                        :rxOn="server.rx_on"
                        :name="server.name"
                        :txUser="server.tx_user"
                        :latency="server.latency"></audioserver>
                      <div class="list-group-separator"></div>
                    </div
                  </div>
                </div>
              </div>`,
  props: {
    servers: Object,
  },
  components: {
    'audioserver': AudioServer,
  },
  mounted: function () {},
  beforeDestroy: function () {},
  methods: {
    setAudioServer: function (audioServerName) {
      this.$emit('set-audioserver', audioServerName)
    },
    setRxState: function (audioServerName, rxState) {
      this.$emit('set-rxstate', audioServerName, rxState);
    },
  },
  computed: {},
  watch: {},
}