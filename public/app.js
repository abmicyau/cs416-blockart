var app = new Vue({
    el: '#app',
    data: {
        greeting: 'Welcome to your Vue.js app!',
        CanvasX: 0,
        CanvasY: 0,
    },
    created: function() {
        this.$http.get('/getCanvas').then(function(response) {
            console.log(response)
            this.CanvasX = response.body.X
            this.CanvasY = response.body.Y
        })
        setInterval(function() {
            this.getBlocks();
        }.bind(this), 2000);
    },
    methods: {
        getBlocks: function() {
            this.$http.get('/getBlocks').then(function(response) {
                console.log("hi")
                console.log(response)
            })
        }
    },
})