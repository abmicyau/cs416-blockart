var app = new Vue({
    el: '#app',
    data: {
        CanvasX: 0,
        CanvasY: 0,
        BlockChain: [],
        Shapes: [],
        indexBlockChain: 0,
        blocksWithShapes: [],
    },
    created: function() {
        this.$http.get('/getCanvas').then(function(response) {
            console.log(response)
            this.CanvasX = response.body.X
            this.CanvasY = response.body.Y
        })
        this.initGetBlocks();
        document.getElementById('blocksScroll').addEventListener('scroll', this.handleScroll);
        console.log($('#blocksScroll').get(0).scrollWidth)
        setInterval(function() {
            this.getBlocks();
        }.bind(this), 5000);
    },
    methods: {
        handleScroll: function(e) {
            var thisEle = document.getElementById('blocksScroll');
            var viewWidth = thisEle.clientWidth;
            var sLeft = document.getElementById('blocksScroll').scrollLeft;
            var maxWidth = document.getElementById('blocksScroll').scrollWidth - 766

            var defaultNumberShapes = Math.floor(viewWidth / 270);
            //console.log("Default Shapes " + defaultNumberShapes)
            var totalShapes = Math.floor((sLeft + 270 * defaultNumberShapes) / 270);
            //console.log("totalShapes: " + totalShapes)
            if (totalShapes > defaultNumberShapes) {
                this.Shapes = [];
                if (sLeft == maxWidth) {
                    for (var i = 0; i < this.blocksWithShapes.length; i++) {
                        for (var j = 0; j < this.blocksWithShapes[i].Shapes.length; j++) {
                            this.Shapes.push(this.blocksWithShapes[i].Shapes[j])
                        }
                    }
                } else {
                    for (var i = 0; i <= totalShapes; i++) {
                        for (var j = 0; j < this.blocksWithShapes[i].Shapes.length; j++) {
                            this.Shapes.push(this.blocksWithShapes[i].Shapes[j])
                        }
                    }
                }
            } else {
                this.Shapes = [];
                for (var i = 0; i < defaultNumberShapes; i++) {
                    for (var j = 0; j < this.blocksWithShapes[i].Shapes.length; j++) {
                        this.Shapes.push(this.blocksWithShapes[i].Shapes[j])
                    }
                }
            }
            // console.log(sLeft)
            // console.log(document.getElementById('blocksScroll').scrollWidth)
        },
        getBlocks: function() {
            this.$http.get('/getBlocks').then(function(response) {
                //console.log(response)
                this.BlockChain = response.body.Blocks
                this.blocksWithShapes = []
                for (var i = 0; i < this.BlockChain.length; i++) {
                    if (this.BlockChain[i].Shapes.length > 0) {
                        this.blocksWithShapes.push(this.BlockChain[i])
                    }
                }
            })
        },
        initGetBlocks: function() {
            this.$http.get('/getBlocks').then(function(response) {
                //console.log(response)
                this.BlockChain = response.body.Blocks
                for (var i = 0; i < this.BlockChain.length; i++) {
                    for (var j = 0; j < this.BlockChain[i].Shapes.length; j++) {
                        this.Shapes.push(this.BlockChain[i].Shapes[j])
                    }
                }
                this.indexBlockChain = 5
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