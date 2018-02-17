/*

A trivial application to illustrate how the blockartlib library can be
used from an application in project 1 for UBC CS 416 2017W2.

Usage:
go run art-app.go
*/

package main

// Expects blockartlib.go to be in the ./blockartlib/ dir, relative to
// this art-app.go file
import (
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"proj1_b0z8_b4n0b_i5n8_m9r8/blockartlib"
)

type CanvasSets struct {
	X uint32 `json:"X"`
	Y uint32 `json:"Y"`
}

type BlockJson struct {
	BlockHash string   `json: "BlockHash"`
	Shapes    []string `json: Shapes`
}

type LongestChainJson struct {
	Blocks []BlockJson `json: "Blocks"`
}

var canvasSets CanvasSets
var canvasGlobal blockartlib.Canvas
var lastLongestHash string
var longestChainLength int

func main() {
	addrs, _ := net.InterfaceAddrs()
	var externalIP string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				externalIP = ipnet.IP.String()
			}
		}
	}

	webserverAddr := externalIP + ":8080"
	args := os.Args[1:]

	if len(args) != 2 {
		log.Fatalln("Missing args, Usage: go run art-app_web.go [inkMiner privKey] [inkMiner addr]")
	}
	minerAddr := args[1]

	// Proper Key Generate
	privBytes, _ := hex.DecodeString(args[0])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Private Key")
	}
	// Open a canvas.
	canvas, setting, err := blockartlib.OpenCanvas(minerAddr, *privKey)
	if checkError(err) != nil {
		return
	}

	canvasGlobal = canvas

	canvasSets = *new(CanvasSets)
	canvasSets.X = setting.CanvasXMax
	canvasSets.Y = setting.CanvasYMax

	fmt.Println("Listening on: ", webserverAddr)
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/getCanvas", CanvasHandler)
	http.HandleFunc("/getBlocks", BlocksHandler)
	http.HandleFunc("/getBlocksInit", InitBlocksHandler)
	http.ListenAndServe(webserverAddr, nil)
}

func CanvasHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(canvasSets)
}

func InitBlocksHandler(w http.ResponseWriter, r *http.Request) {
	genHash, _ := canvasGlobal.GetGenesisBlock()
	var blockHashes []string
	blockHashes, _ = getChildren(genHash)

	lengthOfChain := len(blockHashes)

	log.Println("AllBlockHashes: ", blockHashes)
	log.Println("length of longestChain: ", lengthOfChain)

	LongestChainJson := *new(LongestChainJson)
	LongestChainJson.Blocks = make([]BlockJson, lengthOfChain)

	for iBlock, blockHash := range blockHashes {

		shapeHashes, _ := canvasGlobal.GetShapes(blockHash)

		LongestChainJson.Blocks[iBlock].BlockHash = blockHash
		LongestChainJson.Blocks[iBlock].Shapes = make([]string, len(shapeHashes))

		for iShape, shapeHash := range shapeHashes {
			svgString, _ := canvasGlobal.GetSvgString(shapeHash)
			if len(svgString) > 0 {
				LongestChainJson.Blocks[iBlock].Shapes[iShape] = svgString
			}
		}
		if iBlock == len(blockHashes)-1 {
			lastLongestHash = blockHash
		}
	}
	log.Println("Last Longest Hash: ", lastLongestHash)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(LongestChainJson)
}

func BlocksHandler(w http.ResponseWriter, r *http.Request) {
	genHash, _ := canvasGlobal.GetGenesisBlock()
	var blockHashes []string
	if len(lastLongestHash) == 0 {
		blockHashes, _ = getChildren(genHash)
	} else {
		blockHashes, _ = getChildren(lastLongestHash)
		blockHashes = blockHashes[1:]
	}

	lengthOfChain := len(blockHashes)

	log.Println("AllBlockHashes: ", blockHashes)
	log.Println("length of longestChain: ", lengthOfChain)

	LongestChainJson := *new(LongestChainJson)
	LongestChainJson.Blocks = make([]BlockJson, lengthOfChain)

	for iBlock, blockHash := range blockHashes {

		shapeHashes, _ := canvasGlobal.GetShapes(blockHash)

		LongestChainJson.Blocks[iBlock].BlockHash = blockHash
		LongestChainJson.Blocks[iBlock].Shapes = make([]string, len(shapeHashes))

		for iShape, shapeHash := range shapeHashes {
			svgString, _ := canvasGlobal.GetSvgString(shapeHash)
			if len(svgString) > 0 {
				LongestChainJson.Blocks[iBlock].Shapes[iShape] = svgString
			}
		}
		if iBlock == len(blockHashes)-1 {
			lastLongestHash = blockHash
		}
	}
	log.Println("Last Longest Hash: ", lastLongestHash)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(LongestChainJson)
}

// Recursive function to get all the branch chains from Genesis block
func getChildren(genHash string) ([]string, int) {
	hashes, _ := canvasGlobal.GetChildren(genHash)
	if len(hashes) == 0 {
		var hashArray []string
		hashArray = append(hashArray, genHash)
		return hashArray, 1
	}
	var hashArrayForloop []string
	var currLongestLength int
	var currLongestChain []string
	for _, hash := range hashes {
		childArray, lenChildArray := getChildren(hash)
		if lenChildArray > currLongestLength {
			currLongestLength = lenChildArray
			currLongestChain = childArray
		}
	}
	hashArrayForloop = append(hashArrayForloop, genHash)
	hashArrayForloop = append(hashArrayForloop, currLongestChain...)
	return hashArrayForloop, currLongestLength + 1
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
