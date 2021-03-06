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
            var that = this
            jQuery.ajax({
                url: '/getBlocks',
                success: function(response) {
                    console.log(response)
                        //this.BlockChain = response.body.Blocks
                    for (var i = 0; i < response.Blocks.length; i++) {
                        that.BlockChain.push(response.Blocks[i])
                        if (response.Blocks[i].length > 0) {
                            that.blockswithShapes.push(response.Blocks[i])
                        }
                    }
                    var that_that = that;
                    setTimeout(function() {
                        that_that.getBlocks();
                    }, 5000);
                },
                async: false
            });

            // this.$http.get('/getBlocks').then(function(response) {
            //     console.log(response)
            //         //this.BlockChain = response.body.Blocks
            //     for (var i = 0; i < response.body.Blocks.length; i++) {
            //         this.BlockChain.push(response.body.Blocks[i])
            //         if (response.body.Blocks[i].length > 0) {
            //             this.blockswithShapes.push(response.body.Blocks[i])
            //         }
            //     }

            //     // for (var i = 0; i < this.BlockChain.length; i++) {
            //     //     if (this.BlockChain[i].Shapes.length > 0) {
            //     //         this.blocksWithShapes.push(this.BlockChain[i])
            //     //     }
            //     // }
            //     var that = this;
            //     setTimeout(function() {
            //         this.getBlocks();
            //     }, 5000);

        },
        initGetBlocks: function() {
            this.$http.get('/getBlocksInit').then(function(response) {
                console.log(response)
                this.BlockChain = response.body.Blocks
                for (var i = 0; i < this.BlockChain.length; i++) {
                    for (var j = 0; j < this.BlockChain[i].Shapes.length; j++) {
                        this.Shapes.push(this.BlockChain[i].Shapes[j])
                    }
                }
                var that = this;
                setTimeout(function() {
                    that.getBlocks();
                }, 5000);
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