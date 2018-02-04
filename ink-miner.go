/*
An ink miner that can be used in BlockArt

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"log"
	"net"
	"net/rpc"
	"os"
)

var (
	logger     *log.Logger
	localAddr  string
	serverAddr string
	miners     []*rpc.Client // will probably change this to an array of Miner structs, just using connection for now
	minerAddrs []string
	blockChain map[string]string // will change this to a Block type  once block generation is done
	// pubKey
	// privKey
)

type RpcMiners rpc.Client

func main() {
	Init()
	ListenForMiners(EstablishLocalListener())
	// ConnectToServer(localAddr, serverAddr)
	// Server.Call("<listener>.GetNodes", pubKey, &minerAddrs)
	// ConnectToMiners(minerAddrs)

	if len(minerAddrs) > 0 {
		testRPC()
	}

	for {

	}
}

// TESTING FUNCTION, ONLY POC FOR FLOODING PROTOCOL, TODO: DELETE
func testRPC() {
	ConnectToMiners(minerAddrs)
	var isValid bool
	for _, miner := range miners {
		miner.Call("RpcMiners.SendBlock", "hi1234", &isValid)
	}
}

// Initializes the logger, args, and other global variables that will be used
func Init() {
	logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
	args := os.Args[1:]
	serverAddr = args[0]
	blockChain = make(map[string]string)
	// ONLY POC FOR FLOODING PROTOCOL, ADDS MANUALLY MINER ADDRESSES TODO: DELETE
	if len(args) > 1 {
		for _, arg := range args[1:] {
			minerAddrs = append(minerAddrs, arg)
		}
	}

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

//////////////////////////////////////////////////////////////////////////////////////////////////
// < RPC CODE >

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
