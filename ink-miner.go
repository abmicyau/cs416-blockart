/*
An ink miner that can be used in BlockArt

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
    "fmt"
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
	logger     *log.Logger
	localAddr  string
	serverAddr string
	// will probably change this to an array of Miner structs, just using connection for now
	miners                    []*rpc.Client
	blockchain                map[string]*Block
	longestChainLastBlockHash string
	genesisHash               string
	nHashZeroes               uint32
}

type Block struct {
	BlockNo  uint32
	PrevHash string
	Records  []OperationRecord
	PubKey   string
	Nonce    uint32
}

type Operation struct {
	Type        OpType
	ShapeHash   string
	ValidateNum uint8
}

type OperationRecord struct {
	Op     Operation
	OpSig  string
	PubKey string
}

// </TYPE DECLARATIONS>
////////////////////////////////////////////////////////////////////////////////////////////

var (
	logger *log.Logger
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
}

func (m *Miner) listenRPC() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	miner := new(Miner)
	rpc.Register(miner)

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
		block := &Block{blockNo, prevHash, make([]OperationRecord, 0), "<pubKey>", nonce}
		encodedBlock, err := json.Marshal(block)
		if err != nil {
			panic(err)
		}
		blockHash := computeBlockHash(encodedBlock)
		if strings.HasSuffix(blockHash, strings.Repeat("0", int(m.nHashZeroes))) {
			logger.Println(block, blockHash)
			m.blockchain[blockHash] = block
			m.longestChainLastBlockHash = blockHash
			return
		} else {
			nonce++
		}
	}
}

////////////////////////////////////////////////////////////////////////////////////////////
// <RPC METHODS>

// Placeholder to prevent the compile warning
func (m *Miner) Hello(arg string, _ *struct{}) error {
    return nil
}

// </RPC METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <HELPER METHODS>

// Computes the hash of the given block
func computeBlockHash(block []byte) string {
	h := md5.New()
	value := []byte(block)
	h.Write(value)
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

// </HELPER METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
