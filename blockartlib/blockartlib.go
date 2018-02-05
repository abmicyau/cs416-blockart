/*

This package specifies the application's interface to the the BlockArt
library (blockartlib) to be used in project 1 of UBC CS 416 2017W2.

*/

package blockartlib

import (
	"fmt"
	"crypto/ecdsa"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/rpc"
	"os"
)

// Represents a type of shape in the BlockArt system.
type ShapeType int
const (
	// Path shape.
	PATH ShapeType = iota

	// Circle shape (extra credit).
	// CIRCLE
)

// Represents the type of operation for a shape on the canvas
type OpType int
const (
	ADD OpType = iota
	REMOVE
)

// Represents error codes for miner-side shape validation
type ShapeError int
const (
	NO_ERROR ShapeError = iota
	INSUFFICIENT_INK
	INVALID_SHAPE_SVG_STRING
	SHAPE_SVG_STRING_TOO_LONG
	SHAPE_OVERLAP
	OUT_OF_BOUNDS
)


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
	PrivKey   ecdsa.PrivateKey
	Settings  CanvasSettings
	Shapes    map[string]Shape
}

type Shape struct {
	ShapeType      ShapeType
	ShapeSvgString string
	Fill           string
	Stroke         string
	Owner          ecdsa.PublicKey
}

type Operation struct {
	Type        OpType
	Shape       Shape
	ValidateNum uint8
}

type OperationRecord struct {
	Op     Operation
	OpSig  string
	PubKey ecdsa.PublicKey
}

type Block struct {
	BlockNo  uint32
	PrevHash string
	Records  []OperationRecord
	PubKey   ecdsa.PublicKey
	Nonce    uint32
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
// Can return the following errors:
// - DisconnectedError
func OpenCanvas(minerAddr string, privKey ecdsa.PrivateKey) (canvas Canvas, setting CanvasSettings, err error) {
	miner, err := rpc.Dial("tcp", minerAddr)

	if err != nil {
		return CanvasInstance{}, CanvasSettings{}, DisconnectedError(minerAddr)
	}

	canvas = CanvasInstance{minerAddr, miner, privKey, setting, make(map[string]Shape)}

	// Need to initiate some kind of verification with the miner
	// Idea: (since we can't just send the private key of course)
	//       app -> miner    greeting
	//       miner -> app    nonce
	//       app -> miner    signed nonce (w/ private key)
	//       miner -> app    ok

	// TODO: fetch from server
	setting = CanvasSettings{}

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
	shape := Shape{shapeType, shapeSvgString, fill, stroke, c.PrivKey.PublicKey}
	shapeHash = shape.hash()

	blockHash = ""
	inkRemaining = 0

	// TODO: Check for disconnection

	// Validation steps
	//
	// OPTION 1: Do each step separately, some relying on the miner and others
	//           using only private methods
	//
	// TODO: Check for sufficient ink (InsufficientInkError)
	//       -> Request updated ink count from the miner, then do the calculation
	// TODO: Validate the shape produced by the svg string
	//       -> Private method
	// TODO: Validate the length of the svg string
	//       -> Private method
	// TODO: Check if the shape overlaps with existing shapes
	//       -> Send the shape to the miner, since the overlap detection
	//          logic should be on the miner (since it will be continuously
	//          checking overlaps for other miners' shapes)
	// TODO: Check that the shape is within bounds
	//       -> Could do this locally with a private method, but could also send
	//          the shape to the miner since it should also have that logic for
	//          validating other miners' shapes
	//
	//
	// OPTION 2: Send the shape to the miner and have it perform all the validation
	//
	// TODO: Send the shape to the miner and prepare a response object to detect
	//       the appropriate error
	//
	// We'll probably go with option 2 and use the ShapeError type to interpret the error

	return shapeHash, blockHash, inkRemaining, nil
}

// Returns the encoding of the shape as an svg string.
// Can return the following errors:
// - DisconnectedError
// - InvalidShapeHashError
func (c CanvasInstance) GetSvgString(shapeHash string) (svgString string, err error) {
	err = c.Miner.Call("Miner.GetSvgString", shapeHash, &svgString)
	if checkError(err) != nil {
		return "", DisconnectedError(c.MinerAddr)
	}

	// We can interpret an empty svg string to mean an invalid shape hash
	if len(svgString) == 0 {
		return "", InvalidShapeHashError(shapeHash)
	}

	return svgString, nil
}

// Returns the amount of ink currently available.
// Can return the following errors:
// - DisconnectedError
func (c CanvasInstance) GetInk() (inkRemaining uint32, err error) {
	// TODO
	return 0, nil
}

// Removes a shape from the canvas.
// Can return the following errors:
// - DisconnectedError
// - ShapeOwnerError
func (c CanvasInstance) DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error) {
	// TODO
	return 0, nil
}

// Retrieves hashes contained by a specific block.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
func (c CanvasInstance) GetShapes(blockHash string) (shapeHashes []string, err error) {
	// TODO
	return make([]string, 0), nil
}

// Returns the block hash of the genesis block.
// Can return the following errors:
// - DisconnectedError
func (c CanvasInstance) GetGenesisBlock() (blockHash string, err error) {
	// TODO
	return "", nil
}

// Retrieves the children blocks of the block identified by blockHash.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
func (c CanvasInstance) GetChildren(blockHash string) (blockHashes []string, err error) {
	// TODO
	return make([]string, 0), nil
}

// Closes the canvas/connection to the BlockArt network.
// - DisconnectedError
func (c CanvasInstance) CloseCanvas() (inkRemaining uint32, err error) {
	// TODO
	return 0, nil
}

// </EXPORTED METHODS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <PRIVATE METHODS>

func (c CanvasInstance) hasOverlappingShape(shape Shape) bool {
	// check overlap against shapes in c.shapes which are not owned by this app
	return false
}

func (s Shape) hash() string {
	encodedShape, err := json.Marshal(s)
	if checkError(err) != nil {
		return ""
	}

	h := md5.New()
	h.Write(encodedShape)

	return hex.EncodeToString(h.Sum(nil))
}

func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

// </PRIVATE METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
