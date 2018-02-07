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
	"time"
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

// Used to send heartbeat to the server just shy of 1 second each beat
const TIME_BUFFER uint32 = 500

type Miner struct {
	logger                    *log.Logger
	localAddr                 net.Addr
	serverAddr                string
	serverConn                *rpc.Client
	miners                    map[string]*rpc.Client
	blockchain                map[string]*Block
	longestChainLastBlockHash string
	genesisHash               string
	nHashZeroes               uint32
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
	shapes                    map[string]*Shape
	settings                  MinerSettings
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
	Address net.Addr
	Key     ecdsa.PublicKey
}

type BlockAndHash struct {
	Blck      Block
	BlockHash string
}

type ChainAndLength struct {
	BlockChain       []Block
	LongestBlockHash string
}

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

var (
	logger *log.Logger
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&net.TCPAddr{})
	miner := new(Miner)
	miner.init()
	go miner.listenRPC()
	miner.registerWithServer()
	go miner.getMiners()

	// TODO: Get Nodes State Machine

	//miner.minerAddrs = append(miner.minerAddrs, "127.0.0.1:46563") // for manual adding of miners right now
	//miner.minerAddrs = append(miner.minerAddrs, "127.0.0.1:40883")

	miner.getLongestChain()
	for {
		miner.mineNoOpBlock()
	}
}

func (m *Miner) init() {
	args := os.Args[1:]
	m.serverAddr = args[0]
	m.blockchain = make(map[string]*Block)
	m.miners = make(map[string]*rpc.Client)
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
	m.localAddr = listener.Addr()
	logger.Println("Listening on: ", listener.Addr().String())
	rpc.Register(m)
	for {
		conn, err := listener.Accept()
		checkError(err)
		logger.Println("New connection from " + conn.RemoteAddr().String())
		go rpc.ServeConn(conn)
	}
}

// Ink miner registers their address and public key to the server and starts sending heartbeats
func (m *Miner) registerWithServer() {
	serverConn, err := rpc.Dial("tcp", m.serverAddr)
	settings := new(MinerNetSettings)
	err = serverConn.Call("RServer.Register", &MinerInfo{m.localAddr, m.pubKey}, &settings)
	if checkError(err) != nil {
		//TODO: Crashing for now, will need to revisit if there is any softer way to handle the error
		log.Fatal("Couldn't Register to Server")
	}
	m.serverConn = serverConn
	m.settings = settings.MinerSettings
	go m.startHeartBeats()
}

// Sends heartbeats every half second to the server to maintain connection
func (m *Miner) startHeartBeats() {
	var ignored bool
	m.serverConn.Call("RServer.HeartBeat", m.pubKey, &ignored)
	for {
		time.Sleep(time.Duration(m.settings.HeartBeat-TIME_BUFFER) * time.Millisecond)
		m.serverConn.Call("RServer.HeartBeat", m.pubKey, &ignored)
	}
}

// Gets miners from server if below MinNumMinerConnections
func (m *Miner) getMiners() {
  var addrSet []net.Addr
	for minerAddr, minerCon := range m.miners {
		var isConnected bool
		minerCon.Call("Miner.PingMiners", "", &isConnected)
		if !isConnected {
			delete(m.miners, minerAddr)
		}
	}
	if len(m.miners) < int(m.settings.MinNumMinerConnections) {
		m.serverConn.Call("RServer.GetNodes", m.pubKey, &addrSet)
		m.connectToMiners(addrSet)
		logger.Println(addrSet, m.miners)
	}
}

// Establishes RPC connections with miners in addrs array
func (m *Miner) connectToMiners(addrs []net.Addr) {
	for _, minerAddr := range addrs {
		if m.miners[minerAddr.String()] == nil {
			minerConn, err := rpc.Dial("tcp", minerAddr.String())
			checkError(err)
			m.miners[minerAddr.String()] = minerConn
		}
	}
}

func (m *Miner) getLongestChain() {
	longestChainAndLength := new(ChainAndLength)
	for _, minerCon := range m.miners {
		var ignored bool
		chainAndLength := new(ChainAndLength)
		minerCon.Call("Miner.GetBlockChain", ignored, &chainAndLength)
		if len(chainAndLength.BlockChain) > len(longestChainAndLength.BlockChain) {
			longestChainAndLength = chainAndLength
		}
	}
	if len(longestChainAndLength.LongestBlockHash) > 1 {
		currHash := longestChainAndLength.LongestBlockHash
		for i := 0; i < len(longestChainAndLength.BlockChain); i++ {
			// Should be from Latest block to Earliest/Genesis
			m.blockchain[currHash] = &longestChainAndLength.BlockChain[i]
			currHash = longestChainAndLength.BlockChain[i].PrevHash
		}
		m.longestChainLastBlockHash = longestChainAndLength.LongestBlockHash
		logger.Println("Start mining at blockNo: ", m.blockchain[m.longestChainLastBlockHash].BlockNo+1)
	}
}

// Creates a noOp block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func (m *Miner) mineNoOpBlock() {
	var nonce uint32 = 0
	var prevHash string
	var blockNo uint32

	if m.longestChainLastBlockHash == "" {
		prevHash = m.settings.GenesisBlockHash
		blockNo = 0
	} else {
		prevHash = m.longestChainLastBlockHash
		blockNo = m.blockchain[prevHash].BlockNo + 1
	}

	// TODO: When a new block comes that adds to longest chain, stop mining and switch longest chain
	for {
		block := &Block{blockNo, prevHash, make([]OperationRecord, 0), m.pubKey, nonce}
		encodedBlock, err := json.Marshal(block)
		if err != nil {
			panic(err)
		}
		blockHash := md5Hash(encodedBlock)
		if strings.HasSuffix(blockHash, strings.Repeat("0", int(m.settings.PoWDifficultyNoOpBlock))) {
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
	logger.Println("Received Block: ", blockAndHash.BlockHash)

	// TODO:
	//		Validate Block
	//		If Valid, add to block chain
	//		Else return invalid

	// If new block, disseminate
	if _, exists := m.blockchain[blockAndHash.BlockHash]; !exists {
		m.blockchain[blockAndHash.BlockHash] = &blockAndHash.Blck
		// compute longest chain
		newChain := lengthLongestChain(blockAndHash.BlockHash, m.blockchain)
		oldChain := lengthLongestChain(m.longestChainLastBlockHash, m.blockchain)
		if newChain > oldChain {
			m.longestChainLastBlockHash = blockAndHash.BlockHash
		}
		// TODO: Else, reply back with our longest chain to sync up with sender

		//		Disseminate Block to connected Miners
		m.disseminateToConnectedMiners(&blockAndHash)
	}
	return nil
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateToConnectedMiners(blockAndHash *BlockAndHash) {
	m.getMiners() // checks all miners, connects to more if needed
	for minerAddr, minerCon := range m.miners {
		var isConnected bool
		var isValid bool
		minerCon.Call("Miner.PingMiners", "", &isConnected)
		if isConnected {
			minerCon.Call("Miner.SendBlock", blockAndHash, &isValid)
		} else {
			delete(m.miners, minerAddr)
		}
	}
}
// Pings all miners currently listed in the miner map
// If a connected miner fails to reply, that miner should be removed from the map
func (m *Miner) PingMiners(payload string, reply *bool) error {
	*reply = true
	return nil
}

func (m *Miner) GetBlockChain(_ignored bool, chainAndLength *ChainAndLength) error {
	logger.Println("GetBlockChain")
	if len(m.longestChainLastBlockHash) < 1 {
		return nil
	}

	longestChainLength := m.blockchain[m.longestChainLastBlockHash].BlockNo + 1
	longestChain := make([]Block, longestChainLength)

	var currhash = m.longestChainLastBlockHash
	for i := 0; i < int(longestChainLength); i++ {
		longestChain[i] = *m.blockchain[currhash]
		currhash = m.blockchain[currhash].PrevHash
		// if block, exists := m.blockchain[currhash]; exists {
		// 	longestChain = append(longestChain, *block)
		// 	currhash = block.PrevHash
		// 	length++
		// } else {
		// 	break
		// }
	}
	chainAndLength.LongestBlockHash = m.longestChainLastBlockHash
	chainAndLength.BlockChain = longestChain

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
	if len(blockhash) < 1 {
		return length
	}
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
