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
	"math/big"
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

// Represents error codes for requests to an ink miner
type MinerResponseError int

const (
	NO_ERROR MinerResponseError = iota
	INVALID_SIGNATURE
	INVALID_TOKEN
	INSUFFICIENT_INK
	INVALID_SHAPE_SVG_STRING
	SHAPE_SVG_STRING_TOO_LONG
	SHAPE_OVERLAP
	OUT_OF_BOUNDS
	INVALID_SHAPE_HASH
	INVALID_BLOCK_HASH
)

type MinerResponse struct {
	Error   MinerResponseError
	Payload []interface{}
}

type ArtnodeRequest struct {
	Token   string
	Payload []interface{}
}

// Settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32
	CanvasYMax uint32
}

// Settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string

	// The minimum number of ink miners that an ink miner should be
	// connected to. If the ink miner dips below this number, then
	// they have to retrieve more nodes from the server using
	// GetNodes().
	MinNumMinerConnections uint8

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32
	InkPerNoOpBlock uint32

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8
	PoWDifficultyNoOpBlock uint8

	// Canvas settings
	CanvasSettings CanvasSettings
}

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
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
	shapes                    map[string]*Shape
	inkRemaining              uint32
	settings                  *MinerNetSettings
	nonces                    map[string]bool
	tokens                    map[string]bool
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
	InkCost     uint32
	ValidateNum uint8
}

type OperationRecord struct {
	Op     Operation
	OpSig  string
	PubKey ecdsa.PublicKey
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
	logger   *log.Logger
	alphabet = []rune("0123456789abcdef")
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&net.TCPAddr{})
	miner := new(Miner)
	miner.init()
	miner.listenRPC()
	miner.registerWithServer()
	miner.getMiners()
	miner.getLongestChain()
	//logger = log.SetPrefix("[Mining]\n")
	for {
		miner.mineNoOpBlock()
	}
}

func (m *Miner) init() {
	args := os.Args[1:]
	m.serverAddr = args[0]
	m.blockchain = make(map[string]*Block)
	m.shapes = make(map[string]*Shape)
	m.nonces = make(map[string]bool)
	m.tokens = make(map[string]bool)
	m.miners = make(map[string]*rpc.Client)
	if len(args) <= 1 {
		priv := generateNewKeys()
		m.privKey = priv
		m.pubKey = priv.PublicKey
	}
	m.newLongestChain = make(chan bool)
}

func (m *Miner) listenRPC() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)
	rpc.Register(m)
	m.localAddr = listener.Addr()
	logger.Println("Listening on: ", listener.Addr().String())
	go func() {
		for {
			conn, err := listener.Accept()
			checkError(err)
			logger.Println("New connection from " + conn.RemoteAddr().String())
			go rpc.ServeConn(conn)
		}
	}()
}

// Ink miner registers their address and public key to the server and starts sending heartbeats
func (m *Miner) registerWithServer() {
	serverConn, err := rpc.Dial("tcp", m.serverAddr)
	settings := new(MinerNetSettings)
	err = serverConn.Call("RServer.Register", &MinerInfo{m.localAddr, m.pubKey}, settings)
	if checkError(err) != nil {
		//TODO: Crashing for now, will need to revisit if there is any softer way to handle the error
		log.Fatal("Couldn't Register to Server")
	}
	m.serverConn = serverConn
	m.settings = settings
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
	for {
		select {
		case <-m.newLongestChain:
			logger.Println("Got a new longest chain!")
			prevHash = m.longestChainLastBlockHash
			blockNo = m.blockchain[prevHash].BlockNo + 1
		default:
			block := &Block{blockNo, prevHash, make([]OperationRecord, 0), m.pubKey, nonce}
			encodedBlock, err := json.Marshal(block)
			if err != nil {
				panic(err)
			}
			blockHash := md5Hash(encodedBlock)
			if strings.HasSuffix(blockHash, strings.Repeat("0", int(m.settings.PoWDifficultyNoOpBlock))) {
				logger.Println(block, blockHash)
				m.blockchain[blockHash] = block
				logger.Println(m.blockchain)
				m.longestChainLastBlockHash = blockHash
				m.update(block)
				blockAndHash := &BlockAndHash{*block, blockHash}
				m.disseminateToConnectedMiners(*blockAndHash)
				return
			} else {
				nonce++
			}
		}
	}
}

// For performance reasons, we keep track of some state in the miner.
// This is not strictly necessary, but it will make many operations much
// easier to perform.
//
// This method updates the miner's state based in a newly mined block.
// This includes:
//  - Adding new shapes to the shape collection
//  - Removing deleted shapes from the shape collection
//  - Updating the amount of ink that the miner has remaining
//
// Miner.update() has a complimentary function, Miner.revert(). See below.
//
// ! Assumption: an ADD and REMOVE operation for the same shape will
// not exist in the same block.
//
// TODO #1: Remember that any time we switch blockchains (for example,
// the if current one is outrun), we must reverse all operations up
// to the most recent ancestor and re-apply state changes that can be
// derived from the new chain.
//
// TODO #2: Use a lock to ensure thread safety.
//
func (m *Miner) update(block *Block) {
	// update shapes and ink per operation
	for _, record := range block.Records {
		op := record.Op
		if op.Type == ADD {
			m.shapes[op.ShapeHash] = &op.Shape
			if record.PubKey == m.pubKey {
				m.inkRemaining -= op.InkCost
			}
		} else {
			delete(m.shapes, op.ShapeHash)
			if record.PubKey == m.pubKey {
				m.inkRemaining += op.InkCost
			}
		}
	}

	// add ink for the newly mined block if it was mined by this miner
	if block.PubKey == m.pubKey {
		if len(block.Records) == 0 {
			m.inkRemaining += m.settings.InkPerNoOpBlock
		} else {
			m.inkRemaining += m.settings.InkPerOpBlock
		}
	}
}

// Reverses update(). Used to roll back during a branch switch.
//
func (m *Miner) revert(block *Block) {
	for _, record := range block.Records {
		op := record.Op
		if op.Type == REMOVE {
			m.shapes[op.ShapeHash] = &op.Shape
			if record.PubKey == m.pubKey {
				m.inkRemaining -= op.InkCost
			}
		} else {
			delete(m.shapes, op.ShapeHash)
			if record.PubKey == m.pubKey {
				m.inkRemaining += op.InkCost
			}
		}
	}

	if block.PubKey == m.pubKey {
		if len(block.Records) == 0 {
			m.inkRemaining -= m.settings.InkPerNoOpBlock
		} else {
			m.inkRemaining -= m.settings.InkPerOpBlock
		}
	}
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateToConnectedMiners(blockAndHash BlockAndHash) {
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

////////////////////////////////////////////////////////////////////////////////////////////
// <RPC METHODS>

func (m *Miner) Hello(_ string, nonce *string) error {
	*nonce = getRand256()
	m.nonces[*nonce] = true
	return nil
}

// Once a token is successfully retrieved, that nonce can no longer be used
//
func (m *Miner) GetToken(request *ArtnodeRequest, response *MinerResponse) error {
	nonce := request.Payload[0].(string)
	r := new(big.Int)
	s := new(big.Int)
	r, r_ok := r.SetString(request.Payload[1].(string), 0)
	s, s_ok := s.SetString(request.Payload[2].(string), 0)

	if !r_ok || !s_ok {
		response.Error = INVALID_SIGNATURE
		return nil
	}

	_, validNonce := m.nonces[nonce]
	validSignature := ecdsa.Verify(&m.pubKey, []byte(nonce), r, s)

	if validNonce && validSignature {
		delete(m.nonces, nonce)
		response.Error = NO_ERROR
		response.Payload = make([]interface{}, 2)
		token := getRand256()
		m.tokens[token] = true

		response.Payload[0] = token
		response.Payload[1] = m.settings.CanvasSettings
	} else {
		response.Error = INVALID_SIGNATURE
	}

	return nil
}

// Get the svg string for the shape identified by a given shape hash, if it exists
func (m *Miner) GetSvgString(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = INVALID_TOKEN
		return nil
	}

	hash := request.Payload[0].(string)
	shape := m.shapes[hash]
	if shape == nil {
		response.Error = INVALID_SHAPE_HASH
		return nil
	}

	response.Error = NO_ERROR
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = shape.ShapeSvgString
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
			m.newLongestChain <- true
		} // TODO: else, if equal, pick the largest hash = random
		// TODO: Else, reply back with our longest chain to sync up with sender

		//		Disseminate Block to connected Miners
		m.disseminateToConnectedMiners(blockAndHash)
	}
	return nil
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

// Get the amount of ink remaining associated with the miners pub/priv key pair
func (m *Miner) GetInk(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = INVALID_TOKEN
		return nil
	}

	response.Error = NO_ERROR
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.inkRemaining

	return nil
}

// Get the hash of the genesis block
func (m *Miner) GetGenesisBlock(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = INVALID_TOKEN
		return nil
	}

	response.Error = NO_ERROR
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.settings.GenesisBlockHash

	return nil
}

// Get a list of shape hashes in a given block
func (m *Miner) GetShapes(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = INVALID_TOKEN
		return nil
	}

	hash := request.Payload[0].(string)
	block := m.blockchain[hash]
	if block == nil {
		response.Error = INVALID_BLOCK_HASH
		return nil
	}

	response.Error = NO_ERROR
	response.Payload = make([]interface{}, 1)
	shapeHashes := make([]string, len(block.Records))
	for i, record := range block.Records {
		shapeHashes[i] = record.Op.ShapeHash
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

// Generates a secure 256-bit nonce/token string for
// artnode request authentication.
//
func getRand256() string {
	str := make([]rune, 64)
	maxIndex := big.NewInt(int64(len(alphabet)))
	for i := range str {
		index, _ := rand.Int(rand.Reader, maxIndex)
		str[i] = alphabet[index.Int64()]
	}
	return string(str)
}

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
