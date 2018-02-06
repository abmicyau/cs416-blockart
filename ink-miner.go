/*
An ink miner that can be used in BlockArt

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////////////////
// <TYPE DECLARATIONS>

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota
)

// Represents the type of operation for a shape on the canvas
type OpType int

const (
	ADD OpType = iota
	REMOVE
)

type Miner struct {
	logger                    *log.Logger
	localAddr                 net.TCPAddr
	serverAddr                string
	miners                    []*rpc.Client
	blockchain                map[string]*Block
	longestChainLastBlockHash string
	genesisHash               string
	nHashZeroes               uint32
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
	shapes                    map[string]*Shape
	minerAddrs                []string
}

type Block struct {
	BlockNo  uint32
	PrevHash string
	Records  []OperationRecord
	PubKey   ecdsa.PublicKey
	Nonce    uint32
}

type Shape struct {
	ShapeType      ShapeType
	ShapeSvgString string
	Fill           string
	Stroke         string
	Owner          ecdsa.PublicKey
}

type Operation struct {
	Type        OpType
	Shape       Shape
	ShapeHash   string
	ValidateNum uint8
}

type OperationRecord struct {
	Op     Operation
	OpSig  string
	PubKey ecdsa.PublicKey
}

// Settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32 `json:"canvas-x-max"`
	CanvasYMax uint32 `json:"canvas-y-max"`
}

type MinerSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string `json:"genesis-block-hash"`

	// The minimum number of ink miners that an ink miner should be
	// connected to.
	MinNumMinerConnections uint8 `json:"min-num-miner-connections"`

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32 `json:"ink-per-op-block"`
	InkPerNoOpBlock uint32 `json:"ink-per-no-op-block"`

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32 `json:"heartbeat"`

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8 `json:"pow-difficulty-op-block"`
	PoWDifficultyNoOpBlock uint8 `json:"pow-difficulty-no-op-block"`
}

// Settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	MinerSettings

	// Canvas settings
	CanvasSettings CanvasSettings `json:"canvas-settings"`
}

type MinerInfo struct {
	Address net.TCPAddr
	Key     ecdsa.PublicKey
}

type BlockAndHash struct {
	Blck      Block
	BlockHash string
}

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

var (
	logger *log.Logger
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	gob.Register(&elliptic.CurveParams{})
	miner := new(Miner)
	miner.init()
	go miner.listenRPC()
	miner.registerWithServer()

	miner.minerAddrs = append(miner.minerAddrs, "127.0.0.1:42309")
	miner.connectToMiners()
	for {
		miner.mineNoOpBlock()
	}
}

func (m *Miner) init() {
	args := os.Args[1:]
	m.serverAddr = args[0]
	m.nHashZeroes = uint32(5)
	m.genesisHash = "01234567890123456789012345678901"
	m.blockchain = make(map[string]*Block)
	logger.Println(m.blockchain)
	if len(args) <= 1 {
		priv := generateNewKeys()
		m.privKey = priv
		m.pubKey = priv.PublicKey
	}
}

func (m *Miner) listenRPC() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)
	m.localAddr = *tcpAddr

	logger.Println("Listening on: ", listener.Addr().String())

	miner := new(Miner)
	rpc.Register(miner)

	for {
		conn, err := listener.Accept()
		checkError(err)
		logger.Println("New connection from " + conn.RemoteAddr().String())
		go rpc.ServeConn(conn)
	}
}

func (m *Miner) registerWithServer() {
	serverConn, err := rpc.Dial("tcp", m.serverAddr)
	settings := new(MinerNetSettings)
	err = serverConn.Call("RServer.Register", &MinerInfo{m.localAddr, m.pubKey}, &settings)
	logger.Println(err)
	logger.Println(settings)
}

func (m *Miner) connectToMiners() {
	for _, minerAddr := range m.minerAddrs {
		minerConn, err := rpc.Dial("tcp", minerAddr)
		checkError(err)
		m.miners = append(m.miners, minerConn)
	}
}

// Creates a noOp block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func (m *Miner) mineNoOpBlock() {
	var nonce uint32 = 0
	var prevHash string
	var blockNo uint32

	if m.longestChainLastBlockHash == "" {
		prevHash = m.genesisHash
		blockNo = 0
	} else {
		prevHash = m.longestChainLastBlockHash
		blockNo = m.blockchain[prevHash].BlockNo + 1
	}

	for {
		block := &Block{blockNo, prevHash, make([]OperationRecord, 0), m.pubKey, nonce}
		encodedBlock, err := json.Marshal(block)
		if err != nil {
			panic(err)
		}
		blockHash := md5Hash(encodedBlock)
		if strings.HasSuffix(blockHash, strings.Repeat("0", int(m.nHashZeroes))) {
			logger.Println(block, blockHash)
			m.updateShapes(block)
			m.blockchain[blockHash] = block
			logger.Println(m.blockchain)
			m.longestChainLastBlockHash = blockHash
			blockAndHash := &BlockAndHash{*block, blockHash}
			for _, minerCon := range m.miners {
				var isValid bool
				minerCon.Call("Miner.SendBlock", blockAndHash, &isValid)
			}
			return
		} else {
			nonce++
		}
	}
}

// Updates the miner's shape collection for a newly mined block.
// AddShape operations add a shape to the collection and DeleteShape
// operations remove them.
//
// Assumption: an ADD and REMOVE operation for the same shape will
// not exist in the same block.
//
// TODO: Remember that any time we switch blockchains (for example,
// the if current one is outrun), we must reverse all operations up
// to the most recent ancestor and re-apply the updated shapes in
// the new chain. We must also remember to re-create the shape
// collection if the miner newly joins the network, or upon recovery
// from a disconnection/failure.
//
func (m *Miner) updateShapes(block *Block) {
	for _, record := range block.Records {
		op := record.Op
		if op.Type == ADD {
			m.shapes[op.ShapeHash] = &op.Shape
		} else {
			delete(m.shapes, op.ShapeHash)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////////////////
// <RPC METHODS>

// Placeholder to prevent the compile warning
func (m *Miner) Hello(arg string, _ *struct{}) error {
	return nil
}

func (m *Miner) SendBlock(blockAndHash BlockAndHash, isValid *bool) error {
	logger.SetPrefix("[SendBlock()]\n")
	logger.Println("Received Block: ", blockAndHash.BlockHash)

	// TODO:
	//		Validate Block
	//		If Valid, add to block chain
	//		Else return invalid

	// If new block, disseminate
	if _, exists := m.blockchain[blockAndHash.BlockHash]; !exists {
		logger.Println(m.blockchain)
		m.blockchain[blockAndHash.BlockHash] = &blockAndHash.Blck
		// compute longest chain
		newChain := lengthLongestChain(blockAndHash.BlockHash, m.blockchain)
		oldChain := lengthLongestChain(m.longestChainLastBlockHash, m.blockchain)
		if newChain > oldChain {
			m.longestChainLastBlockHash = blockAndHash.BlockHash
		}
		//		Disseminate Block to connected Miners
		for _, minerCon := range m.miners {
			var isValid bool
			minerCon.Call("Miner.SendBlock", blockAndHash, &isValid)
		}
	}
	return nil
}

// Get the svg string for the shape identified by a given shape hash, if it exists
func (m *Miner) GetSvgString(hash string, reply *string) error {
	shape := m.shapes[hash]
	if shape != nil {
		*reply = shape.ShapeSvgString
	}
	return nil
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

// Counts the length of the block chain given a block hash
func lengthLongestChain(blockhash string, blockchain map[string]*Block) int {
	var length int
	var currhash = blockhash
	for {
		prevBlockHash := blockchain[currhash].PrevHash
		if _, exists := blockchain[prevBlockHash]; exists {
			currhash = prevBlockHash
			length++
		} else {
			break
		}
	}
	return length
}

// Computes the md5 hash of a given byte slice
func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

func generateNewKeys() ecdsa.PrivateKey {
	c := elliptic.P521()
	privKey, err := ecdsa.GenerateKey(c, rand.Reader)
	checkError(err)
	return *privKey
}

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
