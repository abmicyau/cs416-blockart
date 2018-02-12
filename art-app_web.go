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

var canvasSets CanvasSets
var canvasGlobal blockartlib.Canvas

func main() {
	minerAddr := "127.0.0.1:37005"
	webserverAddr := "127.0.0.1:8080"

	args := os.Args[1:]
	//minerAddr := args[0]

	// Proper Key Generate
	privBytes, _ := hex.DecodeString(args[1])
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
	allBlockHashes := getChildren(genHash)
	log.Println(allBlockHashes)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(canvasSets)
}

func getChildren(genHash string) []string {
	hashes, _ := canvasGlobal.GetChildren(genHash)
	if len(hashes) == 0 {
		var hashArray []string
		hashArray = append(hashArray, genHash)
		return hashArray
	}
	var hashArrayForloop []string
	for _, hash := range hashes {
		childArray := getChildren(hash)
		hashArrayForloop = append(hashArrayForloop, childArray...)
	}
	return hashArrayForloop

}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
