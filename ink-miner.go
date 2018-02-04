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
	"net"
	"net/rpc"
	"os"
	"strings"
)

const genesisHash = "01234567890123456789012345678901"

type Operation struct {
	op string
}

type OperationRecord struct {
	Op     Operation
	OpSig  string
	PubKey string
}

type Block struct {
	BlockNo  uint32
	PrevHash string
	Records  []OperationRecord
	PubKey   string
	Nonce    uint32
}

var (
	logger                    *log.Logger
	localAddr                 string
	serverAddr                string
	miners                    []*rpc.Client     // will probably change this to an array of Miner structs, just using connection for now
	blockchain                map[string]*Block = make(map[string]*Block)
	longestChainLastBlockHash string
	nHashZeroes               uint32
	// minerAddrs []string
	// pubKey
	// privKey
)

type RpcMiners rpc.Client

func main() {
	Init()
	ListenForMiners(EstablishLocalListener())
	for {
		MineNoOpBlock(nHashZeroes)
	}
	// ConnectToServer(localAddr, serverAddr)
	// Server.Call("<listener>.GetNodes", pubKey, &minerAddrs)
	// ConnectToMiners(minerAddrs)
}

// Initializes the logger, args, and other global variables that will be used
func Init() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	args := os.Args[1:]
	serverAddr = args[0]
	nHashZeroes = uint32(5)
	// pubKey = args[1]
	// privKey = args[2]
}

// Establishes the server that will listen for incoming connections from other miners
func EstablishLocalListener() net.Listener {
	conn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// STUB - don't necessarily have to handle this with panic
		panic(err)
	}
	localAddr = conn.Addr().String()
	logger.Println("Listening on: ", localAddr)
	return conn
}

// Listens for incoming connections from other miners
func ListenForMiners(conn net.Listener) {
	rpcMinersServer := rpc.NewServer()
	rpcMinersRPC := new(RpcMiners)
	rpcMinersServer.Register(rpcMinersRPC)
	go rpcMinersServer.Accept(conn)
}

func ConnectToMiners(minerAddrs []string) {
	for _, minerAddr := range minerAddrs {
		minerConn, err := rpc.Dial("tcp", minerAddr)
		check(err)
		miners = append(miners, minerConn)
	}
}

// // Establishes connection and registers ink miner to the main server
// func ConnectToServer(localAddr, serverAddr string) {
//   serverConn, err := rpc.Dial("tcp", serverAddr)
//   check(err)
//   serverConn.Call("<listener>.Register", localAddr, &<settings>)
// }

// Checks for error and Prints if there is one
func check(err error) {
	if err != nil {
		logger.Println(err)
	}
}

// Creates a noOp block and block hash that has a suffix of nHashZeroes
// If successful, block is appended to the longestChainLastBlockHashin the blockchain map
func MineNoOpBlock(nHashZeroes uint32) {
	var nonce uint32 = 0
	var prevHash string
	var blockNo uint32

	if longestChainLastBlockHash == "" {
		prevHash = genesisHash
		blockNo = 0
	} else {
		prevHash = longestChainLastBlockHash
		blockNo = blockchain[prevHash].BlockNo + 1
	}

	for {
		block := &Block{blockNo, prevHash, make([]OperationRecord, 0), "<pubKey>", nonce}
		encodedBlock, err := json.Marshal(block)
		if err != nil {
			panic(err)
		}
		blockHash := computeBlockHash(encodedBlock)
		if strings.HasSuffix(blockHash, strings.Repeat("0", int(nHashZeroes))) {
			logger.Println(block, blockHash)
			blockchain[blockHash] = block
			longestChainLastBlockHash = blockHash
			return
		} else {
			nonce++
		}
	}
}

// Computes the hash of the given block
func computeBlockHash(block []byte) string {
	h := md5.New()
	value := []byte(block)
	h.Write(value)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// < RPC CODE >

// Validates a block sent from another miner, if valid and new, disseminate accordingly
func (t *RpcMiners) SendBlock(block string, isValid *bool) error {
	logger.SetPrefix("[SendBlock()]\n")
	logger.Println("Received Block: ", block)
	// TODO:
	//		Validate Block
	//		If Valid, add to block chain
	//		Else return invalid

	// If new block, disseminate
	if _, exists := blockChain[block]; !exists {
		blockChain[block] = block
		//		Disseminate Block to connected Miners
		for _, minerCon := range miners {
			var isValid bool
			minerCon.Call("RpcMiners.SendBlock", block, &isValid)
		}
	}

	return nil
}
