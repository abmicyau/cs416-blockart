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
  logger *log.Logger
  localAddr string
  serverAddr string
  miners []*rpc.Client // will probably change this to an array of Miner structs, just using connection for now
  // minerAddrs []string
  // pubKey
  // privKey
)

func main() {
  Init()
  ListenForMiners(EstablishLocalListener())
  // ConnectToServer(localAddr, serverAddr)
  // Server.Call("<listener>.GetNodes", pubKey, &minerAddrs)
  // ConnectToMiners(minerAddrs)
}

// Initializes the logger, args, and other global variables that will be used
func Init() {
  logger = log.New(os.Stdout, "[Initializing]\n", log.Lshortfile)
  args := os.Args[1:]
  serverAddr = args[0]
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
  return conn
}

// Listens for incoming connections from other miners
func ListenForMiners(conn net.Listener) {
  rpc.Register(conn)
  go rpc.Accept(conn)
}

// func ConnectToMiners(minerAddrs []string) {
//   for _, minerAddr := range minerAddrs {
//     minerConn, err := rpc.Dial("tcp", minerAddr)
//     check(err)
//     miners = append(miners, minerConn)
//   }
// }

// // Establishes connection and registers ink miner to the main server
// func ConnectToServer(localAddr, serverAddr string) {
//   serverConn, err := rpc.Dial("tcp", serverAddr)
//   check(err)
//   serverConn.Call("<listener>.Register", localAddr, &<settings>)
// }
