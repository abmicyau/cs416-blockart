/*
Usage:
go run art-app.go [privKey] [miner ip:port]
*/

package main

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"proj1_b0z8_b4n0b_i5n8_m9r8/blockartlib"
)

type App struct {
	canvas   blockartlib.Canvas
	settings blockartlib.CanvasSettings
	shapes   map[string]string
	blocks   map[string]string
}

type testLogger struct {
	prefix string
}

func NewLogger(prefix string) testLogger {
	return testLogger{prefix: prefix}
}

func (l testLogger) log(message string) {
	fmt.Printf("[%s][%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), l.prefix, message)
}

func (l testLogger) TestResult(description string, success bool) {
	var label string
	if success {
		label = "OK"
	} else {
		label = "ERROR"
	}

	l.log(fmt.Sprintf("%-70s%-10s", description, label))
}

func main() {

	logger := NewLogger("One Art App")

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

	testCase := fmt.Sprintf("Open Canvas('%s')", minerAddr)
	app.canvas, app.settings, err = blockartlib.OpenCanvas(minerAddr, *privKey)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
		return
	}
	logger.TestResult(testCase, true)

	testCase = fmt.Sprintf("Getting Ink from InkMiner")
	ink, err := app.canvas.GetInk()
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	testCase = fmt.Sprintf("Getting Genesis Block from InkMiner")
	genBlock, err := app.canvas.GetGenesisBlock()
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	var shapeType blockartlib.ShapeType
	shapeType = blockartlib.PATH
	shapeSvgString := "M 0 10 H 20"
	fill := "transparent"
	stroke := "red"
	validateNum := uint8(3)

	testCase = fmt.Sprintf("Adding Shape to Canvas")
	shapeHash, blockHash, _, err := app.canvas.AddShape(validateNum, shapeType, shapeSvgString, fill, stroke)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}
	if ink < 1000 {
		testCase = fmt.Sprintf("Adding Shape to Canvas with insufficient ink")
		noInkShapeSvgString := "M 10 10 h 100 v 100 h -100 z"
		blackFill := "black"
		_, _, _, err = app.canvas.AddShape(validateNum, shapeType, noInkShapeSvgString, blackFill, stroke)
		if checkError(err) == nil {
			logger.TestResult(testCase, true)
		} else {
			logger.TestResult(testCase, false)
		}
	}

	testCase = fmt.Sprintf("Adding Same Shape to Canvas")
	_, _, _, err = app.canvas.AddShape(validateNum, shapeType, shapeSvgString, fill, stroke)
	if checkError(err) == nil {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	testCase = fmt.Sprintf("Adding bad Svg Shape to Canvas")
	badsvgString := "asldnk1209jfn12of12e"
	_, _, _, err = app.canvas.AddShape(validateNum, shapeType, badsvgString, fill, stroke)
	if checkError(err) == nil {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	testCase = fmt.Sprintf("Adding Out of bounds Svg Shape to Canvas")
	badsvgString = "M -" + strconv.Itoa(int(app.settings.CanvasXMax)) + " -" + strconv.Itoa(int(app.settings.CanvasYMax)) + " L -" + strconv.Itoa(int(app.settings.CanvasXMax)) + " -" + strconv.Itoa(int(app.settings.CanvasYMax))
	_, _, _, err = app.canvas.AddShape(validateNum, shapeType, badsvgString, fill, stroke)
	if checkError(err) != nil {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	testCase = fmt.Sprintf("Getting Shapes from BlockHash")
	shapeHashes, err := app.canvas.GetShapes(blockHash)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	testCase = fmt.Sprintf("Checking if block contains added Shape")
	var isShapeHere bool
	for _, shape := range shapeHashes {
		if shape == shapeHash {
			isShapeHere = true
			break
		}
	}
	if isShapeHere {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	testCase = fmt.Sprintf("Getting SVG String")
	svgStringMiner, err := app.canvas.GetSvgString(shapeHash)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	testCase = fmt.Sprintf("Testing SVG String is as expected")
	svgStringApp := `<path d="M 0 10 H 20" stroke="red" fill="transparent"/>`
	if svgStringMiner == svgStringApp {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	testCase = fmt.Sprintf("Deleting SVG String")
	_, err = app.canvas.DeleteShape(validateNum, shapeHash)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	testCase = fmt.Sprintf("Get Children of Genesis Block")
	_, err = app.canvas.GetChildren(genBlock)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
	} else {
		logger.TestResult(testCase, true)
	}

	testCase = fmt.Sprintf("Deleting a deleted SVG String")
	_, err = app.canvas.DeleteShape(validateNum, shapeHash)
	if checkError(err) != nil {
		logger.TestResult(testCase, true)
	} else {
		logger.TestResult(testCase, false)
	}

	fmt.Println("Connected to ink miner at " + minerAddr)
	fmt.Println("Canvas is " + fmt.Sprint(app.settings.CanvasXMax) + " by " + fmt.Sprint(app.settings.CanvasYMax))

}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
