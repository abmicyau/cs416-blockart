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
	"net/http"
	"os"

	"./blockartlib"
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

func main() {
	// TO USE ME
	webserverAddr := "127.0.0.1:8080"
	args := os.Args[1:]

	if len(args) != 2 {
		log.Fatalln("Missing args, Usage: go run art-app_web.go [inkMiner privKey] [inkMiner addr]")
	}
	minerAddr := args[1]

	// Proper Key Generate
	privBytes, _ := hex.DecodeString(args[0])
	//pubBytes, _ := hex.DecodeString(args[2])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		log.Fatalln("Error with Private Key")
	}
	//pubBytes, _ := hex.DecodeString(args[1])
	//pubKey, err := x509.ParsePKIXPublicKey(pubBytes)

	// Open a canvas.
	canvas, setting, err := blockartlib.OpenCanvas(minerAddr, *privKey)
	if checkError(err) != nil {
		return
	}

	canvasGlobal = canvas

	canvasSets = *new(CanvasSets)
	canvasSets.X = setting.CanvasXMax
	canvasSets.Y = setting.CanvasYMax
	// _, _ = canvas.GetShapes("")

	// validateNum := 2

	// // Add a line.
	// shapeHash, blockHash, ink, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 0 5", "transparent", "red")
	// if checkError(err) != nil {
	// 	return
	// }

	// // Add another line.
	// shapeHash2, blockHash2, ink2, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 5 0", "transparent", "blue")
	// if checkError(err) != nil {
	// 	return
	// }

	// // Delete the first line.
	// ink3, err := canvas.DeleteShape(validateNum, shapeHash)
	// if checkError(err) != nil {
	// 	return
	// }

	// // assert ink3 > ink2

	// // Close the canvas.
	// ink4, err := canvas.CloseCanvas()
	// if checkError(err) != nil {
	// 	return
	// }
	//http.HandleFunc("/", handler)

	fmt.Println("Listening on: ", webserverAddr)
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/getCanvas", CanvasHandler)
	http.HandleFunc("/getBlocks", BlocksHandler)
	http.ListenAndServe(webserverAddr, nil)
}

func CanvasHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(canvasSets)
}

func BlocksHandler(w http.ResponseWriter, r *http.Request) {
	genHash, _ := canvasGlobal.GetGenesisBlock()
	blockHashes, lengthOfChain := getChildren(genHash)
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
		// Testing path
		// if iBlock == 3 {
		// LongestChainJson.Blocks[iBlock].Shapes = append(LongestChainJson.Blocks[iBlock].Shapes, `<path stroke="#f00" stroke-width="3" d=" M 50,50 L 100 100"/>`)
		// } else if iBlock == 5 {
		// 	LongestChainJson.Blocks[iBlock].Shapes = append(LongestChainJson.Blocks[iBlock].Shapes, `<path d="M10 80 C 40 10, 65 10, 95 80 S 150 150, 180 80" stroke="black" fill="transparent"/>`)

		// }
	}

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
