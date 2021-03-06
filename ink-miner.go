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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"proj1_b0z8_b4n0b_i5n8_m9r8/errorlib"
	"proj1_b0z8_b4n0b_i5n8_m9r8/shapelib"
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
	lock            *sync.RWMutex
	logger          *log.Logger
	localAddr       net.Addr
	serverAddr      string
	serverConn      *rpc.Client
	miners          map[string]*rpc.Client
	blockchain      map[string]*Block
	blockchainHead  string
	blockChildren   map[string][]string
	pubKey          ecdsa.PublicKey
	privKey         ecdsa.PrivateKey
	pubKeyString    string
	inkAccounts     map[string]uint32
	settings        *MinerNetSettings
	nonces          map[string]bool
	tokens          map[string]bool
	newLongestChain bool
	unminedOps      map[string]*OperationRecord
	unvalidatedOps  map[string]*OperationRecord
	validatedOps    map[string]*OperationRecord
	failedOps       map[string]*OperationRecord
	tempOps         map[string]*OperationRecord
}

type Block struct {
	BlockNo      uint32
	PrevHash     string
	Records      []OperationRecord
	PubKeyString string
	Nonce        uint32
}

type Operation struct {
	Type         OpType
	Shape        shapelib.Shape
	Ref          string
	InkCost      uint32
	ValidateNum  uint8
	NumRemaining uint8
	TimeStamp    int64
	Deleted      bool
}

type OperationRecord struct {
	Op           Operation
	OpSig        string
	PubKeyString string
	Error        error
}

type Signature struct {
	R *big.Int
	S *big.Int
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

type BlockchainMap struct {
	Blockchain map[string]*Block
	Lock       sync.RWMutex
}

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

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
	gob.Register(errorLib.InvalidBlockHashError(""))
	gob.Register(errorLib.DisconnectedError(""))
	gob.Register(errorLib.InvalidShapeSvgStringError(""))
	gob.Register(errorLib.ShapeSvgStringTooLongError(""))
	gob.Register(errorLib.InvalidShapeHashError(""))
	gob.Register(errorLib.ShapeOwnerError(""))
	gob.Register(errorLib.OutOfBoundsError{})
	gob.Register(errorLib.ShapeOverlapError(""))
	gob.Register(errorLib.InvalidShapeFillStrokeError(""))
	gob.Register(errorLib.InvalidSignatureError{})
	gob.Register(errorLib.InvalidTokenError(""))
	gob.Register(errorLib.ValidationError(""))
	gob.Register(errorLib.InsufficientInkError(0))
	miner := new(Miner)
	miner.init()
	miner.listenRPC()
	miner.registerWithServer()
	miner.getMiners()
	miner.initBlockchain()
	logger.SetPrefix("[Mining]\n")
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
	m.blockChildren = make(map[string][]string)
	m.nonces = make(map[string]bool)
	m.tokens = make(map[string]bool)
	m.miners = make(map[string]*rpc.Client)
	m.lock = &sync.RWMutex{}
	if len(args) <= 1 {
		logger.Fatalln("Missing keys, please generate with: go run generateKeys.go")
	}

	privBytes, _ := hex.DecodeString(args[2])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Private Key")
	}

	pubKey := decodeStringPubKey(args[1])

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

	m.newLongestChain = false
}

func (m *Miner) listenRPC() {
	addrs, _ := net.InterfaceAddrs()
	var externalIP string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				externalIP = ipnet.IP.String()
			}
		}
	}
	externalIP = externalIP + ":0"
	tcpAddr, err := net.ResolveTCPAddr("tcp", externalIP)
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
			logger.Println("New connection!")
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
	}
}

// Establishes RPC connections with miners in addrs array
func (m *Miner) connectToMiners(addrs []net.Addr) {
	for _, minerAddr := range addrs {
		if m.miners[minerAddr.String()] == nil {
			minerConn, err := rpc.Dial("tcp", minerAddr.String())
			if err != nil {
				log.Println(err)
				delete(m.miners, minerAddr.String())
			} else {
				m.miners[minerAddr.String()] = minerConn
				response := new(MinerResponse)
				request := new(MinerRequest)
				request.Payload = make([]interface{}, 1)
				request.Payload[0] = m.localAddr.String()
				minerConn.Call("Miner.BidirectionalSetup", request, response)
			}
		}
	}
}

// When a new miner joins the network, it'll ask all the neighbouring miners for their longest chain
// After retrieving the chain, it'll use one of them as it's starting chain
// This method will do the following:
//	After returning with a chain
// 	- Validate the shape with existing miner states (ink, exisiting shapes)
//	- Apply the Block's state to the miner to validate future blocks
// 	- Revert the blocks to earse the memory

// After the checks, it'll keep the current longest valid chain
// The new miner will then apply the blocks again and start mining from the end of that chain

func (m *Miner) initBlockchain() {
	m.lock.Lock()
	defer m.lock.Unlock()

	request := new(MinerRequest)

	m.initBlockchainCache()

	// For each connected Miner, get the length of their longest chain first
	mapMinerAndLength := make(map[string]int)
	for minerAddr, minerCon := range m.miners {
		singleResponse := new(MinerResponse)
		minerCon.Call("Miner.GetBlockChainLength", request, singleResponse)
		if len(singleResponse.Payload) > 0 {
			lengthMinerChain := singleResponse.Payload[0].(int)
			mapMinerAndLength[minerAddr] = lengthMinerChain
		}
	}

	sortedMap := sortMap(mapMinerAndLength)
	// Then get go through from highest to lowest
	for _, pair := range sortedMap {
		singleResponse := new(MinerResponse)
		m.miners[pair.Key].Call("Miner.GetBlockChain", request, singleResponse)
		if len(singleResponse.Payload) > 0 {
			currentChain := singleResponse.Payload[0].([]Block)
			isChainValid := true

			// The order of currentChain from low to high indices is newest to oldest, so
			// we have to traverse backwards
			for i := len(currentChain) - 1; i >= 0; i-- {
				block := &currentChain[i]

				// If the block is invalid, the chain is also invalid, so move on to the next chain
				if m.validateBlock(block) != nil {
					isChainValid = false
					break
				}
				// Else, the block is valid, so apply the block to simulate
				m.addBlock(block)
				m.applyBlock(block)
			}

			// If the chain is valid and longer than any other valid chain we've received,
			// then set it as the new longest chain
			if isChainValid {
				logger.Println("Got an existing chain, start mining at blockNo: ", m.blockchain[m.blockchainHead].BlockNo+1)
				break
			}

			// Reset the miner state
			m.initBlockchainCache()
			// otherwise go to the next one
		}
	}
}

func (m *Miner) initBlockchainCache() {
	m.unminedOps = make(map[string]*OperationRecord)
	m.unvalidatedOps = make(map[string]*OperationRecord)
	m.validatedOps = make(map[string]*OperationRecord)
	m.failedOps = make(map[string]*OperationRecord)
	m.tempOps = make(map[string]*OperationRecord)
	m.blockchain = make(map[string]*Block)
	m.inkAccounts = make(map[string]uint32)
	m.inkAccounts[m.pubKeyString] = 0

	genesisBlock := &Block{0, "", []OperationRecord{}, "", 0}
	m.blockchain[m.settings.GenesisBlockHash] = genesisBlock
	m.blockchainHead = m.settings.GenesisBlockHash
}

// Creates a block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func (m *Miner) mineBlock() {
	m.lock.Lock()
	var nonce uint32 = 0
	prevHash := m.blockchainHead
	blockNo := m.blockchain[prevHash].BlockNo + 1
	m.lock.Unlock()

	for {
		m.lock.Lock()
		if m.newLongestChain {
			m.newLongestChain = false
			m.lock.Unlock()
			return
		} else {
			var block Block
			// Will create a opBlock or noOpBlock depending upon whether unminedOps are waiting to be mined
			if len(m.unminedOps) > 0 {
				opRecordArray := make([]OperationRecord, len(m.unminedOps))
				i := 0
				for _, opRecord := range m.unminedOps {
					opRecordArray[i] = *opRecord
					i++
				}
				block = Block{blockNo, prevHash, opRecordArray, m.pubKeyString, nonce}
			} else {
				block = Block{blockNo, prevHash, nil, m.pubKeyString, nonce}
			}
			if m.blockSuccessfullyMined(&block) {
				m.lock.Unlock()
				return
			} else {
				nonce++
			}
		}
		m.lock.Unlock()
	}
}

// Manages miner state updates during a change of the blockchain head.
//
// Notes:
// - When we are only doing a fast-forward, there is no 'oldBranch'. Also, 'newBranch'
//   will only contain one block. Otherwise (if we are switching branches), this will
//   not be the case.
// - The first for-loop constructs part of the (and possibly the entire) newBranch.
// - The second for-loop continues to construct newBranch while at the same time constructing
//   oldBranch, so long as each pair of successive child blocks have the same BlockNo but are
//   different blocks. This continues until the most recent common ancestor is reached, at
//   which point the construction of newBranch and oldBranch will be complete.
//
// In the case of a branch switch, we perform the following procedure (this can also be
// generalized to the simple case of a fast-forward):
// - Traverse the blocks in the old branch one at a time, up to the most
//   recent common ancestor
//     - Update (reverse) ink accounts for each block
//     - In each block, for each operation:
//         - Reverse the ink associated with that operation
//         - Add the operation to the unmined group
//         - Remove the operation from all other groups
// - Traverse the blocks in the new branch one at a time
//     - Apply each block in order, starting at the child of the most recent common ancestor
//     - Note: this MUST be done in order from oldest to newest, because of the way we decrement
//       our validateNum counter. This is why we do a backwards traversal.
//
// Assumption: oldBlockHash and newBlockHash must both be valid block hashes
// for blocks which exist in the miner's current block map, and are both
// connected to the genesis block.
//
func (m *Miner) changeBlockchainHead(oldBlockHash, newBlockHash string) {
	// newBlock and oldBlock are "current" block pointers
	newBlock := m.blockchain[newBlockHash]
	oldBlock := m.blockchain[oldBlockHash]
	// newBranch and oldBranch are chains of blocks in the new and old branches
	// up to the most recent common ancestor.
	newBranch := []*Block{}
	oldBranch := []*Block{}

	// Construct the part of the new branch up to the block with the same BlockNo
	// as the old branch head
	for newBlock.BlockNo > oldBlock.BlockNo {
		newBranch = append(newBranch, newBlock)
		newBlock = m.blockchain[newBlock.PrevHash]
	}

	// Construct the part of the old branch up to the block with the same BlockNo
	// as the new branch head
	for newBlock.BlockNo < oldBlock.BlockNo {
		oldBranch = append(oldBranch, oldBlock)
		oldBlock = m.blockchain[oldBlock.PrevHash]
	}

	// Construct the rest of the new and old branches at the same time, until
	// their pointers are equal.
	for newBlock != oldBlock {
		newBranch = append(newBranch, newBlock)
		oldBranch = append(oldBranch, oldBlock)
		newBlock = m.blockchain[newBlock.PrevHash]
		oldBlock = m.blockchain[oldBlock.PrevHash]
	}

	// Move each operation in the old branch back to the unmined group and reverse
	// ink accounts.
	for _, block := range oldBranch {
		for _, opRecord := range block.Records {
			opRecord.Op.NumRemaining = opRecord.Op.ValidateNum
			m.unminedOps[opRecord.OpSig] = &opRecord
			delete(m.unvalidatedOps, opRecord.OpSig)
			delete(m.validatedOps, opRecord.OpSig)
			m.reverseOpInk(&opRecord)
		}
		m.reverseBlockInk(block)
	}

	// Apply the blocks in the new branch. NOTE THE ORDER IN WHICH THIS IS DONE.
	// Must be oldest -> newest, in order to correctly validate unvalidated ops.
	// If this is done in the correct order, it will also update the blockchainHead.
	for i := len(newBranch) - 1; i >= 0; i-- {
		m.applyBlock(newBranch[i])
	}
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateToConnectedMiners(block *Block) error {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = *block
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			go minerCon.Call("Miner.SendBlock", request, response)
		} else {
			delete(m.miners, minerAddr)
		}
	}
	return nil
}

func (m *Miner) validateNewShape(s shapelib.Shape) (inkCost uint32, err error) {
	canvasSettings := m.settings.CanvasSettings
	_, geo, err := s.IsValid(canvasSettings.CanvasXMax, canvasSettings.CanvasYMax)
	if err != nil {
		return
	} else if inkCost = uint32(geo.GetInkCost()); inkCost > m.inkAccounts[m.pubKeyString] {
		err = errorLib.InsufficientInkError(m.inkAccounts[m.pubKeyString])
		return
	} else {
		// Check against all unmined, unvalidated, and validated operations
		if overlaps, hash := m.hasOverlappingShape(s, geo); overlaps {
			err = errorLib.ShapeOverlapError(hash)
			return
		}
	}
	return
}

func (m *Miner) hasOverlappingShape(s shapelib.Shape, geo shapelib.ShapeGeometry) (overlaps bool, hash string) {
	opCollections := []map[string]*OperationRecord{m.unminedOps, m.unvalidatedOps, m.validatedOps, m.tempOps}

	for _, opCollection := range opCollections {
		for hash, opRecord := range opCollection {
			_s := opRecord.Op.Shape
			if _s.Owner == s.Owner {
				continue
			} else if _geo, _ := _s.GetGeometry(); _geo.HasOverlap(geo) {
				return true, hash
			}
		}
	}

	return false, hash
}

// Adds a block to the current blocktree, without changing any other
// miner state, and disseminates the block to connected miners.
func (m *Miner) addBlock(block *Block) {
	blockHash := hashBlock(block)
	m.blockchain[blockHash] = block
	m.addBlockChild(block)
	m.disseminateToConnectedMiners(block)
}

// This method applies a block's operations to the miner.
// This means that only in THIS function will we change any miner state
// related to unmined, unvalidated, validated, or failed ops, and ink
// accounts for all miners.
//
// Important: This methods sets the blockchainHead! There should be no
// need to set the blockchainHead other than in this method, EXCEPT
// for the genesis block in initBlockchain().
func (m *Miner) applyBlock(block *Block) {
	m.applyBlockAndOpInk(block)
	m.moveUnminedToUnvalidated(block)
	m.moveUnvalidatedToValidated()
	m.blockchainHead = hashBlock(block)
}

// Adds a block's hash to its parent's list of child hashes.
func (m *Miner) addBlockChild(block *Block) {
	hash := hashBlock(block)
	if _, exists := m.blockChildren[block.PrevHash]; !exists {
		m.blockChildren[block.PrevHash] = []string{hash}
	} else {
		children := m.blockChildren[block.PrevHash]
		m.blockChildren[block.PrevHash] = append(children, hash)
	}
}

// Subtracts or credits ink to the ink accounts of each operation owner
// within a specified block, as well as ink for the mined block itself.
//
// TODO: Use a mutex
//
func (m *Miner) applyBlockAndOpInk(block *Block) {
	// update ink per operation
	for _, record := range block.Records {
		m.applyOpInk(&record)
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

func (m *Miner) applyOpInk(opRecord *OperationRecord) (inkRemaining uint32) {
	op := opRecord.Op
	if _, exists := m.inkAccounts[opRecord.PubKeyString]; !exists {
		m.inkAccounts[opRecord.PubKeyString] = 0
	}
	if op.Type == ADD {
		m.inkAccounts[opRecord.PubKeyString] -= op.InkCost
	} else {
		m.inkAccounts[opRecord.PubKeyString] += op.InkCost
	}

	return m.inkAccounts[opRecord.PubKeyString]
}

func (m *Miner) reverseOpInk(opRecord *OperationRecord) {
	op := opRecord.Op
	if op.Type == ADD {
		m.inkAccounts[opRecord.PubKeyString] += op.InkCost
	} else {
		m.inkAccounts[opRecord.PubKeyString] -= op.InkCost
	}
}

func (m *Miner) reverseBlockInk(block *Block) {
	if len(block.Records) == 0 {
		m.inkAccounts[block.PubKeyString] -= m.settings.InkPerNoOpBlock
	} else {
		m.inkAccounts[block.PubKeyString] -= m.settings.InkPerOpBlock
	}
}

func (m *Miner) blockSuccessfullyMined(block *Block) bool {
	blockHash := hashBlock(block)
	if m.hashMatchesPOWDifficulty(blockHash, len(block.Records)) {
		err := m.validateBlock(block)
		if err != nil {
			return false
		}
		logger.Println("Found a new Block. [" + fmt.Sprint(block.BlockNo) + "] [" + blockHash + "]")
		m.addBlock(block)
		m.applyBlock(block)
		time.Sleep(50 * time.Millisecond)
		// logger.Println("Current BlockChainMap: ", m.blockchain)
		return true
	} else {
		return false
	}
}

// Asserts that block hash matches the intended POW difficulty
func (m *Miner) hashMatchesPOWDifficulty(blockHash string, numRecords int) bool {
	if numRecords == 0 {
		return strings.HasSuffix(blockHash, strings.Repeat("0", int(m.settings.PoWDifficultyNoOpBlock)))
	} else {
		return strings.HasSuffix(blockHash, strings.Repeat("0", int(m.settings.PoWDifficultyOpBlock)))
	}
}

// Moves all operations in a newly mined block from the unmined op collection
// to the unvalidated op collection.
func (m *Miner) moveUnminedToUnvalidated(block *Block) {
	for _, opRecord := range block.Records {
		// previously using &opRecord would not work properly when adding multiple
		// records into unvalidated. Deep copy ensures the values exist in that map
		newOpRecord := &OperationRecord{
			Op:           opRecord.Op,
			OpSig:        opRecord.OpSig,
			PubKeyString: opRecord.PubKeyString}
		m.unvalidatedOps[opRecord.OpSig] = newOpRecord
		delete(m.unminedOps, opRecord.OpSig)
		logger.Println("OperationRecord has been placed into a block. [" + opRecord.Op.Shape.ShapeSvgString + "]")
	}
}

// Decrements the validation num counter for each op in the unvalidated op collection
// and moves those which have become valid to the validated op collection
func (m *Miner) moveUnvalidatedToValidated() {
	for _, opRecord := range m.unvalidatedOps {
		if opRecord.Op.NumRemaining <= 0 {
			if opRecord.Op.Type == REMOVE {
				m.validatedOps[opRecord.Op.Ref].Op.Deleted = true
			}
			m.validatedOps[opRecord.OpSig] = opRecord
			delete(m.unvalidatedOps, opRecord.OpSig)
			logger.Println("OperationRecord has been validated. [" + opRecord.Op.Shape.ShapeSvgString + "]")
		} else {
			opRecord.Op.NumRemaining -= 1
			logger.Println("OperationRecord validateNum decreased. [" + fmt.Sprint(opRecord.Op.NumRemaining) + "] [" + opRecord.Op.Shape.ShapeSvgString + "]")
		}
	}
}

// Sends block to all connected miners
// Makes sure that enough miners are connected; if under minimum, it calls for more
func (m *Miner) disseminateOpToConnectedMiners(opRec *OperationRecord) {
	m.getMiners() // checks all miners, connects to more if needed
	request := new(MinerRequest)
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = *opRec
	response := new(MinerResponse)
	for minerAddr, minerCon := range m.miners {
		isConnected := false
		minerCon.Call("Miner.PingMiner", "", &isConnected)
		if isConnected {
			go minerCon.Call("Miner.SendOp", request, response)
		} else {
			delete(m.miners, minerAddr)
		}
	}
}

// </PRIVATE METHODS : MINER>
////////////////////////////////////////////////////////////////////////////////////////////

//

////////////////////////////////////////////////////////////////////////////////////////////
// <RPC METHODS>

func (m *Miner) Hello(_ string, nonce *string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	*nonce = getRand256()
	m.nonces[*nonce] = true
	return nil
}

// Once a token is successfully retrieved, that nonce can no longer be used
//
func (m *Miner) GetToken(request *ArtnodeRequest, response *MinerResponse) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

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
		response.Payload = make([]interface{}, 3)
		token := getRand256()
		m.tokens[token] = true

		response.Payload[0] = token
		response.Payload[1] = m.settings.CanvasSettings.CanvasXMax
		response.Payload[2] = m.settings.CanvasSettings.CanvasYMax
	} else {
		response.Error = new(errorLib.InvalidSignatureError)
	}

	return nil
}

// Gets the svg string for the shape identified by a given shape hash (operation
// signature), if it exists.
//
// This only checks for ops in the validated group (because there's no way an art
// app could get the hash of an unvalidated operation).
//
func (m *Miner) GetSvgString(request *ArtnodeRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	hash := request.Payload[0].(string)
	opRecord := m.validatedOps[hash]
	if opRecord == nil {
		response.Error = errorLib.InvalidShapeHashError(hash)
		return nil
	}

	response.Error = nil
	response.Payload = make([]interface{}, 1)

	shape := opRecord.Op.Shape
	if shape.ShapeType == shapelib.CIRCLE {
		_geo, _ := shape.GetGeometry()
		geo, _ := _geo.(shapelib.CircleGeometry)

		cx := strconv.FormatInt(geo.Center.X, 10)
		cy := strconv.FormatInt(geo.Center.Y, 10)
		r := strconv.FormatInt(geo.Radius, 10)

		response.Payload[0] = `<circle cx="` + cx + `" cy="` + cy + `" r="` + r + `" stroke="` + shape.Stroke + `" fill="` + shape.Fill + `"/>`
	} else {
		response.Payload[0] = `<path d="` + shape.ShapeSvgString + `" stroke="` + shape.Stroke + `" fill="` + shape.Fill + `"/>`
	}

	return nil
}

func (m *Miner) SendBlock(request *MinerRequest, response *MinerResponse) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	block := request.Payload[0].(Block)
	blockHash := hashBlock(&block)

	_, blockExists := m.blockchain[blockHash]
	_, parentExists := m.blockchain[block.PrevHash]

	if blockExists || !parentExists {
		return
	}

	oldBlockchainHead := m.blockchainHead
	m.changeBlockchainHead(oldBlockchainHead, block.PrevHash)
	err = m.validateBlock(&block)
	m.changeBlockchainHead(m.blockchainHead, oldBlockchainHead)

	if err == nil {
		logger.Println("Received new block. [" + fmt.Sprint(block.BlockNo) + "] [" + blockHash + "]")

		m.addBlock(&block)

		newChainLength := block.BlockNo
		oldChainLength := m.blockchain[m.blockchainHead].BlockNo

		if newChainLength > oldChainLength || (newChainLength == oldChainLength && blockHash > m.blockchainHead) {
			logger.Println("Blockchain head changed. Now mining after block [" + fmt.Sprint(newChainLength) + "]")
			m.applyBlock(&block)
			m.validateUnminedOps()
			m.newLongestChain = true
		}
	}

	return
}

func (m *Miner) SendOp(request *MinerRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	opRec := request.Payload[0].(OperationRecord)
	logger.Println("Received Op: ", opRec.OpSig)

	if opRec.Op.Type == ADD {
		if _, shapeError := m.validateNewShape(opRec.Op.Shape); shapeError != nil {
			// The shape being added isn't valid
			return nil
		}
	} else {
		opRecord := m.validatedOps[opRec.Op.Ref]
		if opRecord == nil || opRecord.PubKeyString != opRec.PubKeyString || opRecord.Op.Deleted {
			return nil
		}
	}

	// If new op, disseminate
	_, unminedExists := m.unminedOps[opRec.OpSig]
	_, unvalidExists := m.unvalidatedOps[opRec.OpSig]
	_, validExists := m.validatedOps[opRec.OpSig]
	isSigValid := m.validateSignature(opRec)

	if !unminedExists && !unvalidExists && !validExists && isSigValid {
		m.unminedOps[opRec.OpSig] = &opRec
		m.disseminateOpToConnectedMiners(&opRec)
	}

	return nil
}

// Pings all miners currently listed in the miner map
// If a connected miner fails to reply, that miner should be removed from the map
func (m *Miner) PingMiner(payload string, reply *bool) error {
	*reply = true
	return nil
}

func (m *Miner) GetBlockChainLength(request *MinerRequest, response *MinerResponse) error {
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = int(m.blockchain[m.blockchainHead].BlockNo)
	return nil
}

func (m *Miner) BidirectionalSetup(request *MinerRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	minerAddr := request.Payload[0].(string)
	minerConn, err := rpc.Dial("tcp", minerAddr)
	if err != nil {
		delete(m.miners, minerAddr)
	} else {
		m.miners[minerAddr] = minerConn
		logger.Println("birectional setup complete")
	}
	return nil
}

func (m *Miner) GetBlockChain(request *MinerRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	logger.Println("GetBlockChain")

	longestChainLength := m.blockchain[m.blockchainHead].BlockNo
	if longestChainLength == 0 {
		return nil
	}
	longestChain := make([]Block, longestChainLength)

	var currhash = m.blockchainHead
	for i := 0; i < int(longestChainLength); i++ {
		longestChain[i] = *m.blockchain[currhash]
		currhash = m.blockchain[currhash].PrevHash
	}
	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = longestChain

	return nil
}

// Get the amount of ink remaining associated with the miners pub/priv key pair
func (m *Miner) GetInk(request *ArtnodeRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

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
	m.lock.Lock()
	defer m.lock.Unlock()

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

// Gets a list of shape hashes (operation signatures) in a given block.
//
func (m *Miner) GetShapes(request *ArtnodeRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

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
		shapeHashes[i] = record.OpSig
	}
	response.Payload[0] = shapeHashes

	return nil
}

// Get a list of block hashes which are children of a given block
func (m *Miner) GetChildren(request *ArtnodeRequest, response *MinerResponse) error {
	m.lock.Lock()
	defer m.lock.Unlock()

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
	m.lock.Lock()
	defer m.lock.Unlock()

	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	validateNum := request.Payload[0].(uint8)
	shapeType := shapelib.ShapeType(request.Payload[1].(int))
	shapeSvgString := request.Payload[2].(string)
	fill := strings.Trim(request.Payload[3].(string), " ")
	stroke := strings.Trim(request.Payload[4].(string), " ")

	shape := shapelib.Shape{
		ShapeType:      shapeType,
		ShapeSvgString: shapeSvgString,
		Fill:           fill,
		Stroke:         stroke,
		Owner:          m.pubKeyString}

	inkCost, shapeError := m.validateNewShape(shape)
	if shapeError != nil {
		response.Error = shapeError
		return
	}

	op := Operation{
		Type:         ADD,
		Shape:        shape,
		InkCost:      inkCost,
		ValidateNum:  validateNum,
		NumRemaining: validateNum,
		TimeStamp:    time.Now().UnixNano(),
		Deleted:      false}

	opSig := m.addOperationRecord(&op)

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = opSig

	return
}

func (m *Miner) DeleteShape(request *ArtnodeRequest, response *MinerResponse) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return nil
	}

	shapeHash := request.Payload[0].(string)
	validateNum := request.Payload[1].(uint8)

	opRecord := m.validatedOps[shapeHash]
	if opRecord == nil || opRecord.PubKeyString != m.pubKeyString || opRecord.Op.Deleted {
		response.Error = errorLib.ShapeOwnerError(shapeHash)
		return
	}

	delShape := opRecord.Op.Shape
	inkCost := opRecord.Op.InkCost
	delShape.Fill, delShape.Stroke = "white", "white"

	op := Operation{
		Type:         REMOVE,
		Shape:        delShape,
		Ref:          opRecord.OpSig,
		InkCost:      inkCost,
		ValidateNum:  validateNum,
		NumRemaining: validateNum,
		TimeStamp:    time.Now().UnixNano()}

	opSig := m.addOperationRecord(&op)

	response.Error = nil
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = opSig

	return
}

func (m *Miner) OpValidated(request *ArtnodeRequest, response *MinerResponse) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	opSig := request.Payload[0].(string)
	validOp := m.validatedOps[opSig]
	failedOp := m.failedOps[opSig]

	response.Payload = make([]interface{}, 3)
	response.Payload[0] = false
	response.Payload[1] = ""
	response.Payload[2] = uint32(0)

	if validOp != nil {
		blockHash, err := m.getOpBlockHash(opSig)
		if err != nil {
			response.Error = err
		} else {
			response.Payload[0] = true
			response.Payload[1] = blockHash
			response.Payload[2] = m.inkAccounts[validOp.PubKeyString]
		}
	} else if failedOp != nil {
		response.Error = failedOp.Error
		delete(m.failedOps, opSig)
	} else {
		response.Payload[0] = false
	}

	return
}

func (m *Miner) CloseCanvas(request *ArtnodeRequest, response *MinerResponse) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	token := request.Token
	_, validToken := m.tokens[token]
	if !validToken {
		response.Error = errorLib.InvalidTokenError(token)
		return
	}

	delete(m.tokens, token)
	response.Payload = make([]interface{}, 1)
	response.Payload[0] = m.inkAccounts[m.pubKeyString]

	return
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

//

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

func (m *Miner) addOperationRecord(op *Operation) (opSig string) {
	encodedOp, err := json.Marshal(*op)
	checkError(err)
	r, s, err := ecdsa.Sign(rand.Reader, &m.privKey, encodedOp)
	checkError(err)
	sig := Signature{r, s}
	encodedSig, err := json.Marshal(sig)
	checkError(err)
	opSig = string(encodedSig)

	opRecord := OperationRecord{
		Op:           *op,
		OpSig:        opSig,
		PubKeyString: m.pubKeyString}

	m.unminedOps[opSig] = &opRecord
	m.disseminateOpToConnectedMiners(&opRecord)

	return
}

// Asserts the following about a given block and blockHash:
// - blockhash matches POW difficulty and nonce is correct
// - the given block points to a valid hash in the blockchain
func (m *Miner) validateBlock(block *Block) error {
	blockHash := hashBlock(block)
	if m.hashMatchesPOWDifficulty(blockHash, len(block.Records)) && m.validateOpIntegrity(block) && m.blockchain[block.PrevHash] != nil {
		logger.Println("Block has been validated. [" + fmt.Sprint(block.BlockNo) + "] [" + blockHash + "]")
		return nil
	}
	logger.Println("Block could not be validated. ", blockHash)
	return errorLib.ValidationError(blockHash)
}

// Helper function to assert that each op in a block is signed properly,
// shape is valid, and the public key has enough ink.
func (m *Miner) validateOpIntegrity(block *Block) bool {
	addOps := map[string]*OperationRecord{}
	removeOps := map[string]*OperationRecord{}
	blockValid := true

	// Check for valid signatures and credit ink for REMOVE operations first
	for _, opRecord := range block.Records {
		if !m.validateSignature(opRecord) {
			blockValid = false
		}
		if opRecord.Op.Type == REMOVE {
			removeOps[opRecord.OpSig] = &opRecord
		} else {
			addOps[opRecord.OpSig] = &opRecord
		}
	}

	// Validate each REMOVE operation
	for opSig, opRecord := range removeOps {
		originalOp := m.validatedOps[opRecord.Op.Ref]
		if originalOp == nil || originalOp.Op.Deleted {
			delete(removeOps, opSig)
			blockValid = false
		} else {
			m.applyOpInk(opRecord)
		}
	}

	// Validate each ADD operation
	for opSig, opRecord := range addOps {
		_, err := m.validateNewShape(opRecord.Op.Shape)
		if err != nil {
			logger.Println(err)
			delete(addOps, opSig)
			blockValid = false
		} else {
			m.applyOpInk(opRecord)
			m.tempOps[opSig] = opRecord
		}
	}

	// Clean up tempOps
	m.tempOps = map[string]*OperationRecord{}
	// Reverse temporary inkAccount changes
	for _, opRecord := range removeOps {
		m.reverseOpInk(opRecord)
	}
	for _, opRecord := range addOps {
		m.reverseOpInk(opRecord)
	}

	return blockValid
}

// Validates a the miner's current collection of unmined ops. The shapes
// within the ops are tested for validity and sufficient ink. Ops failing
// validation will be added to the failedOps collection with the error saved.
//
// This is necessary because in the case of a branch switch, unmined ops
// could become invalid when if shapes in the new branch cause conflicts,
// or use up too much ink. This is how ops return as failed.
//
// This assumes that signatures have already been validated (otherwise
// they wouldn't have been added to the unminedOps collection).
//
// ADD operations which conflict with some other unmined operation will be
// successively removed until conflicts no longer occur. The order is
// unspecified.
func (m *Miner) validateUnminedOps() {
	addOps := map[string]*OperationRecord{}
	removeOps := map[string]*OperationRecord{}

	for opSig, opRecord := range m.unminedOps {
		if opRecord.Op.Type == REMOVE {
			removeOps[opSig] = opRecord
		} else {
			addOps[opSig] = opRecord
		}
	}

	// Validate each REMOVE operation and remove if invalid
	for opSig, opRecord := range removeOps {
		originalOp := m.validatedOps[opRecord.Op.Ref]
		if originalOp == nil || originalOp.Op.Deleted {
			opRecord.Error = errorLib.ShapeOwnerError(originalOp.OpSig)
			m.failedOps[opSig] = opRecord
			delete(m.unminedOps, opSig)
		} else {
			m.applyOpInk(opRecord)
		}
	}

	// Validate each ADD operation and remove if invalid
	for opSig, opRecord := range addOps {
		_, err := m.validateNewShape(opRecord.Op.Shape)
		if err != nil {
			opRecord.Error = err
			m.failedOps[opSig] = opRecord
			delete(m.unminedOps, opSig)
		} else {
			m.applyOpInk(opRecord)
		}
	}

	// Reverse temporary inkAccount changes
	for _, opRecord := range m.unminedOps {
		m.reverseOpInk(opRecord)
	}
}

func (m *Miner) validateSignature(opRecord OperationRecord) bool {
	data, _ := json.Marshal(opRecord.Op)
	sig := new(Signature)
	json.Unmarshal([]byte(opRecord.OpSig), &sig)
	return ecdsa.Verify(decodeStringPubKey(opRecord.PubKeyString), data, sig.R, sig.S)
}

func (m *Miner) getOpBlockHash(opSig string) (string, error) {
	hash := m.blockchainHead
	block := m.blockchain[hash]
	blockNo := block.BlockNo
	for blockNo > 1 {
		ops := block.Records
		for _, op := range ops {
			if op.OpSig == opSig {
				return hash, nil
			}
		}

		hash = block.PrevHash
		block = m.blockchain[hash]
		blockNo = block.BlockNo
	}

	return "", errorLib.InvalidShapeHashError(opSig)
}

// </HELPER METHODS>
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

func decodeStringPubKey(pubkey string) *ecdsa.PublicKey {
	pubBytes, _ := hex.DecodeString(pubkey)
	pubKey, err := x509.ParsePKIXPublicKey(pubBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Public Key")
	}
	return pubKey.(*ecdsa.PublicKey)
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

func hashBlock(block *Block) string {
	encodedBlock, err := json.Marshal(*block)
	checkError(err)
	blockHash := md5Hash(encodedBlock)
	return blockHash
}

func sortMap(minerAndLength map[string]int) PairList {
	pl := make(PairList, len(minerAndLength))
	i := 0
	for k, v := range minerAndLength {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Computes the md5 hash of a given byte slice
func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
