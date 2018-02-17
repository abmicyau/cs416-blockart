/*

This package specifies the application's interface to the the BlockArt
library (blockartlib) to be used in project 1 of UBC CS 416 2017W2.

*/

package blockartlib

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"net/rpc"
	"os"
	"time"

	errorLib "../errorlib"
)

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota
	CIRCLE
)

// Represents the type of operation for a shape on the canvas
type OpType int

const (
	ADD OpType = iota
	REMOVE
)

type MinerResponse struct {
	Error   error
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
	canvasSettings CanvasSettings
}

// Represents a canvas in the system.
type Canvas interface {
	// Adds a new shape to the canvas.
	// Can return the following errors:
	// - DisconnectedError
	// - InsufficientInkError
	// - InvalidShapeSvgStringError
	// - ShapeSvgStringTooLongError
	// - ShapeOverlapError
	// - OutOfBoundsError
	AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error)

	// Returns the encoding of the shape as an svg string.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidShapeHashError
	GetSvgString(shapeHash string) (svgString string, err error)

	// Returns the amount of ink currently available.
	// Can return the following errors:
	// - DisconnectedError
	GetInk() (inkRemaining uint32, err error)

	// Removes a shape from the canvas.
	// Can return the following errors:
	// - DisconnectedError
	// - ShapeOwnerError
	DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error)

	// Retrieves hashes contained by a specific block.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidBlockHashError
	GetShapes(blockHash string) (shapeHashes []string, err error)

	// Returns the block hash of the genesis block.
	// Can return the following errors:
	// - DisconnectedError
	GetGenesisBlock() (blockHash string, err error)

	// Retrieves the children blocks of the block identified by blockHash.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidBlockHashError
	GetChildren(blockHash string) (blockHashes []string, err error)

	// Closes the canvas/connection to the BlockArt network.
	// - DisconnectedError
	CloseCanvas() (inkRemaining uint32, err error)
}

type CanvasInstance struct {
	MinerAddr string
	Miner     *rpc.Client
	Token     string
	Closed    *bool
}

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains address IP:port that art node cannot connect to.
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("BlockArt: cannot connect to [%s]", string(e))
}

// Contains amount of ink remaining.
type InsufficientInkError uint32

func (e InsufficientInkError) Error() string {
	return fmt.Sprintf("BlockArt: Not enough ink to addShape [%d]", uint32(e))
}

// Contains the offending svg string.
type InvalidShapeSvgStringError string

func (e InvalidShapeSvgStringError) Error() string {
	return fmt.Sprintf("BlockArt: Bad shape svg string [%s]", string(e))
}

// Contains the offending svg string.
type ShapeSvgStringTooLongError string

func (e ShapeSvgStringTooLongError) Error() string {
	return fmt.Sprintf("BlockArt: Shape svg string too long [%s]", string(e))
}

// Contains the bad shape hash string.
type InvalidShapeHashError string

func (e InvalidShapeHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid shape hash [%s]", string(e))
}

// Contains the bad shape hash string.
type ShapeOwnerError string

func (e ShapeOwnerError) Error() string {
	return fmt.Sprintf("BlockArt: Shape owned by someone else [%s]", string(e))
}

// Empty
type OutOfBoundsError struct{}

func (e OutOfBoundsError) Error() string {
	return fmt.Sprintf("BlockArt: Shape is outside the bounds of the canvas")
}

// Contains the hash of the shape that this shape overlaps with.
type ShapeOverlapError string

func (e ShapeOverlapError) Error() string {
	return fmt.Sprintf("BlockArt: Shape overlaps with a previously added shape [%s]", string(e))
}

// Contains the invalid block hash.
type InvalidBlockHashError string

func (e InvalidBlockHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid block hash [%s]", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <EXPORTED METHODS>

// The constructor for a new Canvas object instance. Takes the miner's
// IP:port address string and a public-private key pair (ecdsa private
// key type contains the public key). Returns a Canvas instance that
// can be used for all future interactions with blockartlib.
//
// The returned Canvas instance is a singleton: an application is
// expected to interact with just one Canvas instance at a time.
//
// The art node will undergo the following registration/verification
// protocol with the ink miner:
//
// 1. ArtNode -> InkMiner  Hello
// 2. InkMiner -> ArtNode  Nonce
// 3. ArtNode -> Inkminer  Sign(Nonce)
// 4. InkMiner -> ArtNode  Token, CanvasSettings
//
// The returned token (if registration is successful) must be included
// in all future API calls.
//
// Can return the following errors:
// - DisconnectedError
func OpenCanvas(minerAddr string, privKey ecdsa.PrivateKey) (canvas Canvas, setting CanvasSettings, err error) {
	// Greet the miner and retrieve a nonce
	gob.Register(errorLib.InvalidBlockHashError(""))
	gob.Register(errorLib.DisconnectedError(""))
	gob.Register(errorLib.InvalidShapeSvgStringError(""))
	gob.Register(errorLib.ShapeSvgStringTooLongError(""))
	gob.Register(errorLib.InvalidShapeHashError(""))
	gob.Register(errorLib.ShapeOwnerError(""))
	gob.Register(errorLib.OutOfBoundsError{})
	gob.Register(errorLib.ShapeOverlapError(""))
	gob.Register(errorLib.InvalidShapeFillStrokeError(""))
	gob.Register(errorLib.InvalidSignatureError{})
	gob.Register(errorLib.InvalidTokenError(""))
	gob.Register(errorLib.ValidationError(""))

	miner, err := rpc.Dial("tcp", minerAddr)
	if checkError(err) != nil {
		return CanvasInstance{}, CanvasSettings{}, DisconnectedError(minerAddr)
	}
	var nonce string
	err = miner.Call("Miner.Hello", "", &nonce)
	if checkError(err) != nil {
		return CanvasInstance{}, CanvasSettings{}, DisconnectedError(minerAddr)
	}

	// Sign the nonce and form a token request
	r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(nonce))
	checkError(err)
	request := new(ArtnodeRequest)
	request.Payload = make([]interface{}, 3)
	request.Payload[0] = nonce
	request.Payload[1] = r.String()
	request.Payload[2] = s.String()

	// Request token and canvas settings from the miner
	response := new(MinerResponse)
	err = miner.Call("Miner.GetToken", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") {
		err = DisconnectedError(minerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	token := response.Payload[0].(string)
	settingX := response.Payload[1].(uint32)
	settingY := response.Payload[2].(uint32)
	setting = CanvasSettings{CanvasXMax: settingX, CanvasYMax: settingY}
	canvas = CanvasInstance{minerAddr, miner, token, &false}

	return canvas, setting, nil
}

// Adds a new shape to the canvas.
// Can return the following errors:
// - DisconnectedError
// - InsufficientInkError
// - InvalidShapeSvgStringError
// - ShapeSvgStringTooLongError
// - ShapeOverlapError
// - OutOfBoundsError
func (c CanvasInstance) AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 5)
	request.Payload[0] = validateNum
	request.Payload[1] = int(shapeType)
	request.Payload[2] = shapeSvgString
	request.Payload[3] = fill
	request.Payload[4] = stroke
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.AddShape", request, response)

	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	shapeHash = response.Payload[0].(string)

	request = new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = shapeHash
	response = new(MinerResponse)
	for {
		err = c.Miner.Call("Miner.OpValidated", request, response)

		validated := response.Payload[0].(bool)
		blockHash = response.Payload[1].(string)
		inkRemaining = response.Payload[2].(uint32)
		if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
			err = DisconnectedError(c.MinerAddr)
			return
		} else if response.Error != nil {
			err = response.Error
			return
		} else if validated == true {
			return
		}

		time.Sleep(time.Second)
	}

	return
}

// Returns the encoding of the shape as an svg string.
// Can return the following errors:
// - DisconnectedError
// - InvalidShapeHashError
//
// TODO: Testing
//
func (c CanvasInstance) GetSvgString(shapeHash string) (svgString string, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = shapeHash
	response := new(MinerResponse)
	err = c.Miner.Call("Miner.GetSvgString", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	svgString = response.Payload[0].(string)

	return svgString, nil
}

// Returns the amount of ink currently available.
// Can return the following errors:
// - DisconnectedError
//
// TODO: Testing
//
func (c CanvasInstance) GetInk() (inkRemaining uint32, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.GetInk", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	inkRemaining = response.Payload[0].(uint32)

	return inkRemaining, nil
}

// Removes a shape from the canvas.
// Can return the following errors:
// - DisconnectedError
// - ShapeOwnerError
func (c CanvasInstance) DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error) {
	request := new(ArtnodeRequest)
	response := new(MinerResponse)
	request.Token = c.Token
	request.Payload = make([]interface{}, 2)
	request.Payload[0] = shapeHash
	request.Payload[1] = validateNum
	err = c.Miner.Call("Miner.DeleteShape", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if errorLib.IsType(response.Error, "ShapeOwnerError") {
		err = ShapeOwnerError(shapeHash)
		return
	}

	opSig := response.Payload[0].(string)

	request = new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = opSig
	response = new(MinerResponse)
	for {
		err = c.Miner.Call("Miner.OpValidated", request, response)

		validated := response.Payload[0].(bool)
		inkRemaining = response.Payload[2].(uint32)

		if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
			err = DisconnectedError(c.MinerAddr)
			return
		} else if response.Error != nil {
			err = response.Error
			return
		} else if validated == true {
			return
		}

		time.Sleep(time.Second)
	}

	return
}

// Retrieves hashes contained by a specific block.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
//
// For now, assume that this call returns all shapes (both add and delete operations)
// No duplicates, because add and remove operations for the same shape can't be in
// the same block.
//
// TODO: Double check these semantics.
//
func (c CanvasInstance) GetShapes(blockHash string) (shapeHashes []string, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = blockHash
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.GetShapes", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	shapeHashes = response.Payload[0].([]string)

	return shapeHashes, nil
}

// Returns the block hash of the genesis block.
// Can return the following errors:
// - DisconnectedError
//
// TODO: Testing
//
func (c CanvasInstance) GetGenesisBlock() (blockHash string, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.GetGenesisBlock", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	blockHash = response.Payload[0].(string)

	return blockHash, nil
}

// Retrieves the children blocks of the block identified by blockHash.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
func (c CanvasInstance) GetChildren(blockHash string) (blockHashes []string, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = blockHash
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.GetChildren", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	} else if response.Error != nil {
		err = response.Error
		return
	}

	blockHashes = response.Payload[0].([]string)
	return blockHashes, nil
}

// Closes the canvas/connection to the BlockArt network.
// - DisconnectedError
func (c CanvasInstance) CloseCanvas() (inkRemaining uint32, err error) {
	request := new(ArtnodeRequest)
	request.Token = c.Token
	request.Payload = make([]interface{}, 1)
	request.Payload[0] = blockHash
	response := new(MinerResponse)

	err = c.Miner.Call("Miner.CloseCanvas", request, response)
	if checkError(err) != nil || errorLib.IsType(response.Error, "InvalidTokenError") || *c.Closed {
		err = DisconnectedError(c.MinerAddr)
		return
	}

	inkRemaining = response.Payload[0].(uint32)
	*c.Closed = true

	return 0, nil
}

// </EXPORTED METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <PRIVATE METHODS>

func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

// </PRIVATE METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
