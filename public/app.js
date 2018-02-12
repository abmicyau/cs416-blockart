var app = new Vue({
    el: '#app',
    data: {
        greeting: 'Welcome to your Vue.js app!',
        CanvasX: 0,
        CanvasY: 0,
        BlockChain: [],
    },
    created: function() {
        this.$http.get('/getCanvas').then(function(response) {
            console.log(response)
            this.CanvasX = response.body.X
            this.CanvasY = response.body.Y
        })
        setInterval(function() {
            this.getBlocks();
        }.bind(this), 5000);
    },
    methods: {
        getBlocks: function() {
            this.$http.get('/getBlocks').then(function(response) {
                console.log(response)
                this.BlockChain = response.body.Blocks
            })
        }
    },
})