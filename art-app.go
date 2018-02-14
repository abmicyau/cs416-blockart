/*
Usage:
go run art-app.go [privKey] [miner ip:port]
*/

package main

import "./blockartlib"

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"bufio"
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
)

type App struct {
	canvas   blockartlib.Canvas
	settings blockartlib.CanvasSettings
	shapes   map[string]string
	blocks   map[string]string
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Println("Usage: go run art-app.go [privKey] [miner ip:port]")
		return
	}

	privBytes, _ := hex.DecodeString(args[0])
	privKey, err := x509.ParseECPrivateKey(privBytes)
	if checkError(err) != nil {
		return
	}

	app := new(App)
	app.shapes = make(map[string]string)
	app.blocks = make(map[string]string)

	minerAddr := args[1]
	app.canvas, app.settings, err = blockartlib.OpenCanvas(minerAddr, *privKey)
	if checkError(err) != nil {
		return
	}

	fmt.Println("Connected to ink miner at " + minerAddr)
	fmt.Println("Canvas is " + fmt.Sprint(app.settings.CanvasXMax) + " by " + fmt.Sprint(app.settings.CanvasYMax))
	app.Prompt()
}

func (app *App) Prompt() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("ArtApp> ")
    	cmd, _ := reader.ReadString('\n')
    	if app.HandleCommand(cmd) == 1 {
    		return
    	}
	}
}

func (app *App) HandleCommand(cmd string) int {
	args := strings.Split(strings.TrimSpace(cmd), ",")

	switch args[0] {
	case "AddShape":
		app.AddShape(args[1:])
	case "GetSvgString":
		app.GetSvgString(args[1:])
	case "GetInk":
		app.GetInk(args[1:])
	case "DeleteShape":
		app.DeleteShape(args[1:])
	case "GetShapes":
		app.GetShapes(args[1:])
	case "GetGenesisBlock":
		app.GetGenesisBlock(args[1:])
	case "GetChildren":
		app.GetChildren(args[1:])
	case "CloseCanvas":
		err := app.CloseCanvas(args[1:])
		if err == nil {
			return 1
		}
	default:
		fmt.Println(" Invalid command.")
	}

	return 0
}

func (app *App) AddShape(args []string) {
	if len(args) < 5 {
		fmt.Println(" AddShape: not enough arguments.")
		return
	}

	validateNum, err := strconv.ParseInt(args[0], 10, 8)
	if err != nil {
		fmt.Println(" AddShape: could not parse validateNum.")
		return
	}

	shapeTypeString := args[1]
	var shapeType blockartlib.ShapeType
	if shapeTypeString == "PATH" {
		shapeType = blockartlib.PATH
	// } else if shapeTypeString == "CIRCLE" {
	// 	shapeType = blockartlib.CIRCLE
	} else {
		fmt.Println(" AddShape: invalid shapeType.")
		return
	}

	shapeSvgString := args[2]
	fill := args[3]
	stroke := args[4]

	shapeHash, blockHash, inkRemaining, err := app.canvas.AddShape(uint8(validateNum), shapeType, shapeSvgString, fill, stroke)
	if err != nil {
		fmt.Println(" AddShape: " + err.Error())
		return
	}

	shapeDoubleHash := md5Hash([]byte(shapeHash))
	blockDoubleHash := md5Hash([]byte(blockHash))

	app.shapes[shapeDoubleHash] = shapeHash
	app.blocks[blockDoubleHash] = blockHash

	fmt.Println(" AddShape: OK!")
	fmt.Println(" AddShape: shapeHash    = " + shapeDoubleHash)
	fmt.Println(" AddShape: blockHash    = " + blockDoubleHash)
	fmt.Println(" AddShape: inkRemaining = " + fmt.Sprint(inkRemaining))
}

func (app *App) GetSvgString(args []string) {
	if len(args) < 1 {
		fmt.Println(" GetSvgString: not enough arguments.")
		return
	}

	shapeDoubleHash := args[0]
	shapeHash, exists := app.shapes[shapeDoubleHash]
	if !exists {
		fmt.Println(" GetSvgString: could not find shapeHash.")
		return
	}

	svgString, err := app.canvas.GetSvgString(shapeHash)
	if err != nil {
		fmt.Println(" GetSvgString: " + err.Error())
		return
	}

	fmt.Println(" GetSvgString: OK!")
	fmt.Println(" GetSvgString: svgString = " + svgString)
}

func (app *App) GetInk(args []string) {
	inkRemaining, err := app.canvas.GetInk()
	if err != nil {
		fmt.Println(" GetInk: " + err.Error())
		return
	}

	fmt.Println(" GetInk: OK!")
	fmt.Println(" GetInk: inkRemaining = " + fmt.Sprint(inkRemaining))
}

func (app *App) DeleteShape(args []string) {
	if len(args) < 2 {
		fmt.Println(" DeleteShape: not enough arguments.")
		return
	}

	validateNum, err := strconv.ParseInt(args[0], 10, 8)
	if err != nil {
		fmt.Println(" DeleteShape: could not parse validateNum.")
		return
	}

	shapeDoubleHash := args[1]
	shapeHash, exists := app.shapes[shapeDoubleHash]
	if !exists {
		fmt.Println(" DeleteShape: could not find shapeHash.")
		return
	}

	inkRemaining, err := app.canvas.DeleteShape(uint8(validateNum), shapeHash)
	if err != nil {
		fmt.Println(" DeleteShape: " + err.Error())
		return
	}

	fmt.Println(" DeleteShape: OK!")
	fmt.Println(" DeleteShape: inkRemaining = " + fmt.Sprint(inkRemaining))
}

func (app *App) GetShapes(args []string) {
	if len(args) < 1 {
		fmt.Println(" GetShapes: not enough arguments.")
		return
	}

	blockDoubleHash := args[0]
	blockHash, exists := app.blocks[blockDoubleHash]
	if !exists {
		fmt.Println(" GetShapes: could not find blockHash.")
		return
	}

	shapeHashes, err := app.canvas.GetShapes(blockHash)
	if err != nil {
		fmt.Println(" GetShapes: " + err.Error())
		return
	}

	fmt.Println(" GetShapes: OK!")
	fmt.Println(" GetShapes: shapeHashes =")
	for _, shapeHash := range shapeHashes {
		shapeDoubleHash := md5Hash([]byte(shapeHash))
		app.shapes[shapeDoubleHash] = shapeHash
		fmt.Println(" GetShapes:  " + shapeDoubleHash)
	}
}

func (app *App) GetGenesisBlock(args []string) {
	blockHash, err := app.canvas.GetGenesisBlock()
	if err != nil {
		fmt.Println(" GetGenesisBlock: " + err.Error())
		return
	}

	blockDoubleHash := md5Hash([]byte(blockHash))
	app.blocks[blockDoubleHash] = blockHash

	fmt.Println(" GetGenesisBlock: OK!")
	fmt.Println(" GetGenesisBlock: blockHash = " + blockDoubleHash)
}

func (app *App) GetChildren(args []string) {
	if len(args) < 1 {
		fmt.Println(" GetChildren: not enough arguments.")
		return
	}

	blockDoubleHash := args[0]
	blockHash, exists := app.blocks[blockDoubleHash]
	if !exists {
		fmt.Println(" GetChildren: could not find blockHash.")
		return
	}

	blockHashes, err := app.canvas.GetChildren(blockHash)
	if err != nil {
		fmt.Println(" GetChildren: " + err.Error())
		return
	}

	fmt.Println(" GetChildren: OK!")
	fmt.Println(" GetChildren: blockHashes =")
	for _, blockHash := range blockHashes {
		blockDoubleHash := md5Hash([]byte(blockHash))
		app.blocks[blockDoubleHash] = blockHash
		fmt.Println(" GetChildren:  " + blockDoubleHash)
	}
}

func (app *App) CloseCanvas(args []string) (err error) {
	inkRemaining, err := app.canvas.CloseCanvas()
	if err != nil {
		fmt.Println(" CloseCanvas: " + err.Error())
		return
	}

	fmt.Println(" CloseCanvas: OK!")
	fmt.Println(" CloseCanvas: inkRemaining = " + fmt.Sprint(inkRemaining))

	return
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}

func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}