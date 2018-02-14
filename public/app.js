var app = new Vue({
    el: '#app',
    data: {
        greeting: 'Welcome to your Vue.js app!',
        CanvasX: 0,
        CanvasY: 0,
        BlockChain: [],
        Shapes: [],
    },
    created: function() {
        this.$http.get('/getCanvas').then(function(response) {
            console.log(response)
            this.CanvasX = response.body.X
            this.CanvasY = response.body.Y
        })
        this.initGetBlocks();
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
        },
        initGetBlocks: function() {
            this.$http.get('/getBlocks').then(function(response) {
                console.log(response)
                this.BlockChain = response.body.Blocks
                for (var i = 0; i < this.BlockChain.length; i++) {
                    for (var j = 0; j < this.BlockChain[i].Shapes.length; j++) {
                        this.Shapes.push(this.BlockChain[i].Shapes[j])
                    }
                }
            })
        },
        filterShapes: function(block) {
            this.Shapes = [];
            for (var j = 0; j < block.Shapes.length; j++) {
                this.Shapes.push(block.Shapes[j])
            }
        },
        resetShapes: function() {
            for (var i = 0; i < this.BlockChain.length; i++) {
                for (var j = 0; j < this.BlockChain[i].Shapes.length; j++) {
                    this.Shapes.push(this.BlockChain[i].Shapes[j])
                }
            }
        }
    },
})