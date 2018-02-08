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
	"encoding/hex"
	"encoding/json"
    "math/big"
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

type Miner struct {
	localAddr                 string
	serverAddr                string
	miners                    []*rpc.Client
	blockchain                map[string]*Block
	longestChainLastBlockHash string
	genesisHash               string
	nHashZeroes               uint32
	pubKey                    ecdsa.PublicKey
	privKey                   ecdsa.PrivateKey
    shapes                    map[string]*Shape
    inkRemaining              uint32
    settings                  MinerNetSettings
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

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

var (
	logger *log.Logger
    alphabet = []rune("0123456789abcdef")
)

func main() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)

	miner := new(Miner)
	miner.init()
	go miner.listenRPC()
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
    m.shapes = make(map[string]*Shape)
    m.nonces = make(map[string]bool)
    m.tokens = make(map[string]bool)

	if len(args) <= 1 {
		priv := generateNewKeys()
		m.privKey = priv
		m.pubKey = priv.PublicKey
	}
}

func (m *Miner) listenRPC() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	rpc.Register(m)

	for {
		conn, err := listener.Accept()
		checkError(err)
		logger.Println("New connection from " + conn.RemoteAddr().String())
		go rpc.ServeConn(conn)
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
		block := &Block{blockNo, prevHash, make([]OperationRecord, 0), ecdsa.PublicKey{}, nonce}
		encodedBlock, err := json.Marshal(block)
		if err != nil {
			panic(err)
		}
		blockHash := md5Hash(encodedBlock)
		if strings.HasSuffix(blockHash, strings.Repeat("0", int(m.nHashZeroes))) {
			logger.Println(block, blockHash)
			m.blockchain[blockHash] = block
			m.longestChainLastBlockHash = blockHash
            m.update(block)
			return
		} else {
			nonce++
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
	c := elliptic.P224()
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
