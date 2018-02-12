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
	"crypto/x509"
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

	"./errorlib"
	"./shapelib"
)

//

////////////////////////////////////////////////////////////////////////////////////////////
// <TYPE DECLARATIONS>

// Represents the type of operation for a shape on the canvas
type OpType int

const (
	ADD OpType = iota
	REMOVE
)

type MinerResponse struct {
	Error   error
	Payload []interface{}
}

type MinerRequest struct {
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
	blockChildren             map[string][]string
	longestChainLastBlockHash string
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
	pubKeyString              string
	shapes                    map[string]*shapelib.Shape
	inkAccounts               map[string]uint32
	settings                  *MinerNetSettings
	nonces                    map[string]bool
	tokens                    map[string]bool
	newLongestChain           chan bool
	unminedOps                map[string]OperationRecord
	unvalidatedOps            map[string]OperationRecord
	validatedOps              map[string]OperationRecord
}

type Block struct {
	BlockNo      uint32
	PrevHash     string
	Records      []OperationRecord
	PubKeyString string
	Nonce        uint32
}

type Operation struct {
	Type        OpType
	Shape       shapelib.Shape
	ShapeHash   string
	InkCost     uint32
	ValidateNum uint8
}

type OperationRecord struct {
	Op           Operation
	OpSig        string
	PubKeyString string
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

//

var (
	logger   *log.Logger
	alphabet = []rune("0123456789abcdef")
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&net.TCPAddr{})
	gob.Register([]Block{})
	gob.Register(Block{})
	gob.Register(Operation{})
	gob.Register(OperationRecord{})
	miner := new(Miner)
	miner.init()
	miner.listenRPC()
	miner.registerWithServer()
	miner.getMiners()
	miner.getLongestChain()
	logger.SetPrefix("[Mining]\n")
	// go miner.testAddOperation() // UNCOMMENT to test op mining - can remove when ops start flowing
	for {
		miner.mineBlock()
	}
}

//

////////////////////////////////////////////////////////////////////////////////////////////
// <PRIVATE METHODS : MINER>

func (m *Miner) init() {
	args := os.Args[1:]
	m.serverAddr = args[0]
	m.blockchain = make(map[string]*Block)
	m.blockChildren = make(map[string][]string)
	m.shapes = make(map[string]*shapelib.Shape)
	m.nonces = make(map[string]bool)
	m.tokens = make(map[string]bool)
	m.miners = make(map[string]*rpc.Client)
	m.inkAccounts = make(map[string]uint32)
	m.unminedOps = make(map[string]OperationRecord)
	m.unvalidatedOps = make(map[string]OperationRecord)
	m.validatedOps = make(map[string]OperationRecord)
	m.inkAccounts[m.pubKeyString] = 0
	if len(args) <= 1 {
		logger.Fatalln("Missing keys, please generate with: go run generateKeys.go")
		// Can uncomment for lazy generate, just uncomment the bottom chunk
		// priv := generateNewKeys()
		// m.privKey = priv
		// m.pubKey = priv.PublicKey
	}
	// Proper Key Generate
	privBytes, _ := hex.DecodeString(args[2])
	//pubBytes, _ := hex.DecodeString(args[2])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Private Key")
	}

	pubKey := m.decodeStringPubKey(args[1])
	// pubKey, err := x509.ParsePKIXPublicKey(pubBytes)
	// if checkError(err) != nil {
	// 	log.Fatalln("Error with Public Key")
	// }

	// Verify if keys are correct
	data := []byte("Hello World")
	r, s, _ := ecdsa.Sign(rand.Reader, privKey, data)
	if !ecdsa.Verify(pubKey, data, r, s) {
		logger.Fatalln("Keys don't match, try again")
	} else {
		logger.Println("Keys are correct and verified")
	}

	m.privKey = *privKey
	m.pubKey = *pubKey
	m.pubKeyString = args[1]
	// End of Proper Key Generation

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
	if checkError(err) != nil {
		log.Fatal("Server is not reachable")
	}
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

func (m *Miner) decodeStringPubKey(pubkey string) *ecdsa.PublicKey {
	pubBytes, _ := hex.DecodeString(pubkey)
	pubKey, err := x509.ParsePKIXPublicKey(pubBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Public Key")
	}
	return pubKey.(*ecdsa.PublicKey)
}

// Gets miners from server if below MinNumMinerConnections
func (m *Miner) getMiners() {
	var addrSet []net.Addr
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if !isConnected {
			delete(m.miners, minerAddr)
		}
	}
	if len(m.miners) < int(m.settings.MinNumMinerConnections) {
		m.serverConn.Call("RServer.GetNodes", m.pubKey, &addrSet)
		m.connectToMiners(addrSet)
		logger.Println("Current set of Addresses and Miners: ", addrSet, m.miners)
	}
}

// Establishes RPC connections with miners in addrs array
func (m *Miner) connectToMiners(addrs []net.Addr) {
	for _, minerAddr := range addrs {
		if m.miners[minerAddr.String()] == nil {
			minerConn, err := rpc.Dial("tcp", minerAddr.String())
			if err != nil {
				delete(m.miners, minerAddr.String())
			} else {
				m.miners[minerAddr.String()] = minerConn
			}
		}
	}
}

func (m *Miner) getLongestChain() {
	//longestChainAndLength := new(ChainAndLength)
	response := new(MinerResponse)
	request := new(MinerRequest)
	for _, minerCon := range m.miners {
		singleResponse := new(MinerResponse)
		minerCon.Call("Miner.GetBlockChain", request, singleResponse)
		if len(response.Payload) < 1 {
			response = singleResponse
		} else if len(singleResponse.Payload[1].([]Block)) > len(response.Payload[1].([]Block)) {
			response = singleResponse
		}
	}
	if len(response.Payload) > 1 {
		longestBlockHash := response.Payload[0].(string)
		longestBlockChain := response.Payload[1].([]Block)
		currHash := longestBlockHash
		for i := 0; i < len(longestBlockChain); i++ {
			// Should be from Latest block to Earliest/Genesis
			block := &longestBlockChain[i]
			m.blockchain[currHash] = block
			m.addBlockChild(block, currHash)
			currHash = longestBlockChain[i].PrevHash
		}
		m.longestChainLastBlockHash = longestBlockHash
		logger.Println("Got an existing chain, start mining at blockNo: ", m.blockchain[m.longestChainLastBlockHash].BlockNo+1)
	}

	// Create a dummy block as the genesis block
	m.blockchain[m.settings.GenesisBlockHash] = &Block{0, "", []OperationRecord{}, m.pubKeyString, 0}
}

// Creates a block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func (m *Miner) mineBlock() {
	var nonce uint32 = 0
	var prevHash string
	var blockNo uint32

	if m.longestChainLastBlockHash == "" {
		prevHash = m.settings.GenesisBlockHash
		blockNo = 1
	} else {
		prevHash = m.longestChainLastBlockHash
		blockNo = m.blockchain[prevHash].BlockNo + 1
	}
	for {
		select {
		case <-m.newLongestChain:
			logger.Println("Got a new longest chain, switching to: ", m.longestChainLastBlockHash)
			prevHash = m.longestChainLastBlockHash
			blockNo = m.blockchain[prevHash].BlockNo + 1
		default:
			var block Block
			// Will create a opBlock or noOpBlock depending upon whether unminedOps are waiting to be mined
			if len(m.unminedOps) > 0 {
				var opRecordArray []OperationRecord
				for _, opRecord := range m.unminedOps {
					opRecordArray = append(opRecordArray, opRecord)
				}
				block = Block{blockNo, prevHash, opRecordArray, m.pubKeyString, nonce}
			} else {
				block = Block{blockNo, prevHash, nil, m.pubKeyString, nonce}
			}
			if m.blockSuccessfullyMined(&block) {
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
//  - Updating the amount of ink that each miner has remaining
//
// ! Assumption: an ADD and REMOVE operation for the same shape will
// not exist in the same block.
//
// TODO: Use a lock to ensure thread safety.
//
func (m *Miner) applyBlock(block *Block) {
	// update shapes and ink per operation
	for _, record := range block.Records {
		op := record.Op
		if _, exists := m.inkAccounts[record.PubKeyString]; !exists {
			m.inkAccounts[record.PubKeyString] = 0
		}
		if op.Type == ADD {
			m.shapes[op.ShapeHash] = &op.Shape
			m.inkAccounts[record.PubKeyString] -= op.InkCost
		} else {
			delete(m.shapes, op.ShapeHash)
			m.inkAccounts[record.PubKeyString] += op.InkCost
		}
	}

	// add ink for the newly mined block
	if _, exists := m.inkAccounts[block.PubKeyString]; !exists {
		m.inkAccounts[block.PubKeyString] = 0
	}
	if len(block.Records) == 0 {
		m.inkAccounts[block.PubKeyString] += m.settings.InkPerNoOpBlock
	} else {
		m.inkAccounts[block.PubKeyString] += m.settings.InkPerOpBlock
	}
}

// Reverses applyBlock(), for rolling back during a branch switch.
//
func (m *Miner) revertBlock(block *Block) {
	for _, record := range block.Records {
		op := record.Op
		if op.Type == REMOVE {
			m.shapes[op.ShapeHash] = &op.Shape
			m.inkAccounts[record.PubKeyString] -= op.InkCost
		} else {
			delete(m.shapes, op.ShapeHash)
			m.inkAccounts[record.PubKeyString] += op.InkCost
		}
	}

	// add ink for the newly mined block
	if len(block.Records) == 0 {
		m.inkAccounts[block.PubKeyString] -= m.settings.InkPerNoOpBlock
	} else {
		m.inkAccounts[block.PubKeyString] -= m.settings.InkPerOpBlock
	}
}

// Manages miner state updates during a branch switch:
//
// 1. A series of applyBlock() and revertBlock() calls are performed up to the
//    most recent ancestor of the current blockchain head and the new (longer)
//    blockchain head so that the miner state reflects that of the new head.
//    Note that this method is also called for the case of a simple fast-
//    forward, where the most recent ancestor will be one of the blocks
//    themself.
//
// 2. The miner's current blockchain head is updated.
//
// Assumption: oldBlockHash and newBlockHash must both be valid block hashes
// for blocks which exist in the miner's current block map, and are both
// connected to the genesis block. If this is not the case, then some bad
// shit is gonna happen.
//
// TODO: Now we REALLY need a lock on the miner. These miner state updates
// are race conditions waiting to happen...
//
func (m *Miner) switchBranches(oldBlockHash, newBlockHash string) {
	// newBlock and oldBlock are "current" block pointers in the new and
	// old blockchain, respectively, as we traverse backwards
	newBlock := m.blockchain[newBlockHash]
	oldBlock := m.blockchain[oldBlockHash]

	for newBlock.BlockNo > oldBlock.BlockNo {
		m.applyBlock(newBlock)
		prevHash := newBlock.PrevHash
		if prevHash == oldBlockHash {
			// In the case of a fast-forward, the previous hash of the new
			// block will eventually be equal to the hash of the old blockchain
			// head. When it reaches that point, we can return, as we are done
			// applying all necessary state updates in the new chain.
			m.longestChainLastBlockHash = newBlockHash
			m.newLongestChain <- true
			return
		}

		newBlock = m.blockchain[prevHash]
	}

	// If we reach this point, that means the block number of the new block is
	// equal to the block number of the old block, but their block hashes are
	// not equal. Therefore, they are on separate branches (of what are now equal
	// length), so we can now make both block pointers move backwards, adjusting
	// the miner state along the way, until their previous hashes are equal. The
	// fact that the loop guard comes at the end accounts for the last traversal
	// step, as well as the edge case where both branches are only of length 1.
	for {
		m.applyBlock(newBlock)
		m.revertBlock(oldBlock)
		newBlock = m.blockchain[newBlock.PrevHash]
		oldBlock = m.blockchain[oldBlock.PrevHash]
		if newBlock.PrevHash == oldBlock.PrevHash {
			break
		}
	}

	m.longestChainLastBlockHash = newBlockHash
	m.newLongestChain <- true
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateToConnectedMiners(block Block, blockHash string) {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 2)
	request.Payload[0] = block
	request.Payload[1] = blockHash
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			minerCon.Call("Miner.SendBlock", request, response)
		} else {
			delete(m.miners, minerAddr)
		}
	}
}

// Adds a block's hash to its parent's list of child hashes.
//
func (m *Miner) addBlockChild(block *Block, hash string) {
	if _, exists := m.blockChildren[block.PrevHash]; !exists {
		m.blockChildren[block.PrevHash] = []string{hash}
	} else {
		children := m.blockChildren[block.PrevHash]
		m.blockChildren[block.PrevHash] = append(children, hash)
	}
}

func (m *Miner) validateNewShape(s shapelib.Shape) (inkCost uint32, err error) {
	if s.Stroke == "" {
		err = errorLib.InvalidShapeFillStrokeError("Shape stroke must be specified")
		return
	} else if s.Fill == "" {
		err = errorLib.InvalidShapeFillStrokeError("Shape fill must be specified")
		return
	} else if s.Stroke == "transparent" || s.Fill == "transparent" {
		err = errorLib.InvalidShapeFillStrokeError("Both fill and stroke cannot be transparent")
		return
	}

	canvasSettings := m.settings.CanvasSettings
	_, geo, err := s.IsValid(canvasSettings.CanvasXMax, canvasSettings.CanvasYMax)
	if err != nil {
		return
	} else if inkCost = uint32(geo.GetInkCost()); inkCost > m.inkAccounts[m.pubKeyString] {
		err = errorLib.InsufficientInkError(m.inkAccounts[m.pubKeyString])
		return
	} else {
		for _sHash, _s := range m.shapes {
			if _s.Owner == s.Owner {
				continue
			} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
				err = errorLib.ShapeOverlapError(_sHash)
				return
			}
		}
	}

	return
}

// </PRIVATE METHODS : MINER>
////////////////////////////////////////////////////////////////////////////////////////////

//

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateOpToConnectedMiners(opRec OperationRecord) {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = opRec
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			minerCon.Call("Miner.SendOp", request, response)
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
func (m *Miner) GetToken(request *ArtnodeRequest, response *MinerResponse) (err error) {
	nonce := request.Payload[0].(string)
	r := new(big.Int)
	s := new(big.Int)
	r, r_ok := r.SetString(request.Payload[1].(string), 0)
	s, s_ok := s.SetString(request.Payload[2].(string), 0)

	if !r_ok || !s_ok {
		response.Error = new(errorLib.InvalidSignatureError)
		return
	}

	_, validNonce := m.nonces[nonce]
	validSignature := ecdsa.Verify(&m.pubKey, []byte(nonce), r, s)

	if validNonce && validSignature {
		delete(m.nonces, nonce)
		response.Error = nil
		response.Payload = make([]interface{}, 2)
		token := getRand256()
		m.tokens[token] = true

		response.Payload[0] = token
		response.Payload[1] = m.settings.CanvasSettings
	} else {
		response.Error = new(errorLib.InvalidSignatureError)
	}

	return nil
}

// Get the svg string for the shape identified by a given shape hash, if it exists
func (m *Miner) GetSvgString(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	shape := m.shapes[hash]
	if shape == nil {
		response.Error = errorLib.InvalidShapeHashError(hash)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = `<path d="` + shape.ShapeSvgString + `" stroke="` + shape.Stroke + `" fill="` + shape.Fill + `"/>`
	return nil
}

func (m *Miner) SendBlock(request *MinerRequest, response *MinerResponse) error {
	logger.Println("Received Block: ", request.Payload[1].(string))

	block := request.Payload[0].(Block)
	blockHash := request.Payload[1].(string)

	// TODO:
	//		Validate Block
	isHashValid := m.validateBlock(block, blockHash)
	m.moveUnminedToUnvalidated(&block) // need to remove unmined ops to stop mining mined ops
	//		If Valid, add to block chain
	//		Else return invalid

	// If new block, disseminate
	if _, exists := m.blockchain[blockHash]; !exists && isHashValid {
		m.blockchain[blockHash] = &block
		m.addBlockChild(&block, blockHash)
		// compute longest chain
		newChainLength := m.lengthLongestChain(blockHash)
		oldChainLength := m.lengthLongestChain(m.longestChainLastBlockHash)
		if newChainLength > oldChainLength {
			if oldChainLength == 0 {
				m.switchBranches(m.settings.GenesisBlockHash, blockHash)
			} else {
				m.switchBranches(m.longestChainLastBlockHash, blockHash)
			}
		}
		// TODO: else, if equal, pick the largest hash = random
		// TODO: Else, reply back with our longest chain to sync up with sender

		//		Disseminate Block to connected Miners
		m.disseminateToConnectedMiners(block, blockHash)
	}
	return nil
}

func (m *Miner) SendOp(request *MinerRequest, response *MinerResponse) error {
	opRec := request.Payload[0].(OperationRecord)
	logger.Println("Received Op: ", opRec.OpSig)
	isSigValid := m.validateOp(opRec)
	// If new block, disseminate
	_, unMinedExists := m.unminedOps[opRec.OpSig]
	_, unValidExists := m.unvalidatedOps[opRec.OpSig]
	_, validExists := m.validatedOps[opRec.OpSig]

	if !unMinedExists && !unValidExists && !validExists && isSigValid {
		m.unminedOps[opRec.OpSig] = opRec

		//	Disseminate Op to connected Miners
		m.disseminateOpToConnectedMiners(opRec)
	}
	return nil
}

// Pings all miners currently listed in the miner map
// If a connected miner fails to reply, that miner should be removed from the map
func (m *Miner) PingMiner(payload string, reply *bool) error {
	*reply = true
	return nil
}

func (m *Miner) GetBlockChain(request *MinerRequest, response *MinerResponse) error {
	logger.Println("GetBlockChain")
	if len(m.longestChainLastBlockHash) < 1 {
		return nil
	}

	longestChainLength := m.blockchain[m.longestChainLastBlockHash].BlockNo
	longestChain := make([]Block, longestChainLength)

	var currhash = m.longestChainLastBlockHash
	for i := 0; i < int(longestChainLength); i++ {
		longestChain[i] = *m.blockchain[currhash]
		currhash = m.blockchain[currhash].PrevHash
	}
	response.Error = nil
	response.Payload = make([]interface{}, 2)
	response.Payload[0] = m.longestChainLastBlockHash
	response.Payload[1] = longestChain

	return nil
}

// Get the amount of ink remaining associated with the miners pub/priv key pair
func (m *Miner) GetInk(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.inkAccounts[m.pubKeyString]

	return nil
}

// Get the hash of the genesis block
func (m *Miner) GetGenesisBlock(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.settings.GenesisBlockHash

	return nil
}

// Get a list of shape hashes in a given block
func (m *Miner) GetShapes(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	block := m.blockchain[hash]
	if block == nil {
		response.Error = errorLib.InvalidBlockHashError(hash)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	shapeHashes := make([]string, len(block.Records))
	for i, record := range block.Records {
		shapeHashes[i] = record.Op.ShapeHash
	}
	response.Payload[0] = shapeHashes

	return nil
}

// Get a list of block hashes which are children of a given block
func (m *Miner) GetChildren(request *ArtnodeRequest, response *MinerResponse) error {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	children, exists := m.blockChildren[hash]
	if !exists {
		response.Error = errorLib.InvalidBlockHashError(hash)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = children

	return nil
}

func (m *Miner) AddShape(request *ArtnodeRequest, response *MinerResponse) (err error) {
	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	//validateNum := request.Payload[0].(uint8)
	shapeType := request.Payload[1].(shapelib.ShapeType)
	shapeSvgString := request.Payload[2].(string)
	fill := strings.Trim(request.Payload[3].(string), " ")
	stroke := strings.Trim(request.Payload[4].(string), " ")

	shape := shapelib.Shape{
		ShapeType:      shapeType,
		ShapeSvgString: shapeSvgString,
		Fill:           fill,
		Stroke:         stroke,
		Owner:          m.pubKeyString}

	inkCost, err := m.validateNewShape(shape)
	response.Error = err
	response.Payload = make([]interface{}, 3)
	response.Payload[0] = ""
	response.Payload[1] = ""
	response.Payload[2] = m.inkAccounts[m.pubKeyString] - inkCost

	return
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

//

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

// Counts the length of the block chain given a block hash
func (m *Miner) lengthLongestChain(blockhash string) int {
	var length int
	if len(blockhash) < 1 {
		return length
	}
	var currhash = blockhash
	for {
		length++
		prevBlockHash := m.blockchain[currhash].PrevHash
		if _, exists := m.blockchain[prevBlockHash]; exists {
			currhash = prevBlockHash
		} else if prevBlockHash == m.settings.GenesisBlockHash {
			break
		} else {
			// Case where the last block in this chain isn't the Genesis one
			return 0
		}
	}
	return length
}

// Asserts the following about a given block and blockHash:
// - blockhash matches POW difficulty and nonce is correct
// - the given block points to a valid hash in the blockchain
// TODO: operation validations
func (m *Miner) validateBlock(block Block, blockHash string) bool {
	encodedBlock, err := json.Marshal(block)
	checkError(err)
	newBlockHash := md5Hash(encodedBlock)
	if m.hashMatchesPOWDifficulty(newBlockHash) && blockHash == newBlockHash && m.blockchain[block.PrevHash] != nil {
		logger.Println("Received Block hashes to correct hash")
		return true
	}
	return false
}

// Asserts the following about a given OperationRecord:
// TODO: shape, ink and valid sig
func (m *Miner) validateOp(opRec OperationRecord) bool {
	return true
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

// TODO: CLEANUP will not need to use this function when using keys from command line
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

func (m *Miner) blockSuccessfullyMined(block *Block) bool {
	encodedBlock, err := json.Marshal(*block)
	checkError(err)
	blockHash := md5Hash(encodedBlock)
	if m.hashMatchesPOWDifficulty(blockHash) {
		logger.Println("Found a new Block!: ", block, blockHash)
		m.blockchain[blockHash] = block
		m.addBlockChild(block, blockHash)
		logger.Println("Current BlockChainMap: ", m.blockchain)
		m.longestChainLastBlockHash = blockHash
		m.applyBlock(block)
		m.disseminateToConnectedMiners(*block, blockHash)
		m.moveUnminedToUnvalidated(block)
		return true
	} else {
		return false
	}
}

// Computes the md5 hash of a given byte slice
func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

// Asserts that block hash matches the intended POW difficulty
func (m *Miner) hashMatchesPOWDifficulty(blockhash string) bool {
	return strings.HasSuffix(blockhash, strings.Repeat("0", int(m.settings.PoWDifficultyNoOpBlock)))
}

func (m *Miner) moveUnminedToUnvalidated(block *Block) {
	for _, opRecord := range block.Records {
		m.unvalidatedOps[opRecord.OpSig] = opRecord
		delete(m.unminedOps, opRecord.OpSig)
	}
}

// UNCOMMENT to test op mining - can remove once ops begin to flow

// func (m *Miner) testAddOperation() {
// 	shape := &Shape{PATH, "svgString", "fill", "stroke", m.pubKey, make([]Command, 1), make([]Point, 1), make([]LineSegment, 1), Point{0, 1}, Point{1, 0}}
// 	op := &Operation{ADD, *shape, "shapehashstring", uint32(20), uint8(3)}
// 	opRecord := &OperationRecord{*op, "some sig", "somekey"}
// 	time.Sleep(time.Second * 5)
// 	m.unminedOps[opRecord.OpSig] = *opRecord
// 	return
// }

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
