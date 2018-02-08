/*

This package specifies the application's interface to the the BlockArt
library (blockartlib) to be used in project 1 of UBC CS 416 2017W2.

*/

package blockartlib

import (
	"crypto/ecdsa"
	"fmt"
)

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota

	// Circle shape (extra credit).
	// CIRCLE
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
	minerAddr string
	privKey   ecdsa.PrivateKey
	settings  CanvasSettings
	shapes    map[string]Shape
}

type Shape struct {
	shapeType      ShapeType
	shapeSvgString string
	fill           string
	stroke         string
	owner          ecdsa.PublicKey

	commands     []Command
	vertices     []Point
	lineSegments []LineSegment
	min          Point
	max          Point
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
	setting = CanvasSettings{} // TODO: fetch from server
	canvas = CanvasInstance{minerAddr, privKey, setting, make(map[string]Shape)}

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
	shape := Shape{
		fill:           fill,
		stroke:         stroke,
		shapeType:      shapeType,
		shapeSvgString: shapeSvgString,
		owner:          c.privKey.PublicKey}
	shapeHash = shape.hash()

	blockHash = ""
	inkRemaining = 0

	shape.evaluateSvgString()
	if valid, err := shape.isValid(c.settings.CanvasXMax, c.settings.CanvasYMax); !valid {
		return shapeHash, blockHash, inkRemaining, err
	} else if c.hasOverlappingShape(shape) {
		return shapeHash, blockHash, inkRemaining, ShapeOverlapError(shapeHash)
	}

	return shapeHash, blockHash, inkRemaining, nil
}

// Returns the encoding of the shape as an svg string.
// Can return the following errors:
// - DisconnectedError
// - InvalidShapeHashError
func (c CanvasInstance) GetSvgString(shapeHash string) (svgString string, err error) {
	// TODO
	return "", nil
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

func (s *Shape) computeInkUsage() (inkUnits uint64) {
	if s.fill == "transparent" {
		for _, lineSegment := range s.lineSegments {
			inkUnits = inkUnits + lineSegment.length()
		}
	} else {

	}

	return
}

func (s *Shape) isValid(xMax uint32, yMax uint32) (valid bool, err error) {
	valid = true

	for _, vertex := range s.vertices {
		if valid = vertex.inBound(xMax, yMax); !valid {
			err = new(OutOfBoundsError)

			return
		}
	}

	if s.fill != "transparent" {
		for i := range s.lineSegments {
			curSeg := s.lineSegments[i]

			for j := range s.lineSegments {
				if i != j && curSeg.intersects(s.lineSegments[j]) == true {
					valid = false
					err = InvalidShapeSvgStringError(s.shapeSvgString)

					return
				}
			}

			if !valid {
				break
			}
		}
	}

	return
}

func (s *Shape) evaluateSvgString() (err error) {
	s.min, s.max = Point{}, Point{}
	if s.commands, err = getCommands(s.shapeSvgString); err != nil {
		return
	}

	vertices := make([]Point, len(s.commands))

	absPos, relPos := Point{0, 0}, Point{0, 0}
	for i := range s.commands {
		_command := s.commands[i]

		switch _command.cmdType {
		case "M":
			absPos.x, absPos.y = _command.x, _command.y
			relPos.x, relPos.y = _command.x, _command.y

			vertices[i] = Point{relPos.x, relPos.y}
		case "H":
			relPos.x = _command.x

			vertices[i] = Point{relPos.x, absPos.y}
		case "V":
			relPos.y = _command.y

			vertices[i] = Point{absPos.x, relPos.y}
		case "L":
			relPos.x, relPos.y = _command.x, _command.y

			vertices[i] = Point{relPos.x, relPos.y}
		case "h":
			relPos.x = relPos.x + _command.x

			vertices[i] = Point{relPos.x, relPos.y}
		case "v":
			relPos.y = relPos.y + _command.y

			vertices[i] = Point{relPos.x, relPos.y}
		case "l":
			relPos.x, relPos.y = relPos.x+_command.x, relPos.y+_command.y

			vertices[i] = Point{relPos.x, relPos.y}
		}

		if relPos.x < s.min.x {
			s.min.x = relPos.x
		} else if relPos.x > s.max.x {
			s.max.x = relPos.x
		}

		if relPos.y < s.min.y {
			s.min.y = relPos.y
		} else if relPos.y > s.max.y {
			s.max.y = relPos.y
		}
	}

	s.vertices = vertices
	s.lineSegments = getLineSegments(vertices)

	return
}

func (s *Shape) hasOverlap(_s Shape) (overlap bool) {
	s.evaluateSvgString()
	_s.evaluateSvgString()

	lineSegments := getLineSegments(s.vertices)
	_lineSegments := getLineSegments(_s.vertices)
	if s.fill == "transparent" && _s.fill == "transparent" {
		for _, _lineSegment := range _lineSegments {
			for _, lineSegment := range lineSegments {
				if overlap = lineSegment.intersects(_lineSegment); overlap {
					break
				}
			}

			if overlap {
				break
			}
		}
	}

	return
}

func (c CanvasInstance) hasOverlappingShape(shape Shape) (overlap bool) {
	shapes := c.shapes
	for i := range shapes {
		_shape := shapes[i]

		if _shape.owner == shape.owner {
			continue
		}

		overlap = _shape.hasOverlap(shape)
		if overlap {
			break
		}
	}

	return
}

func (s Shape) hash() string {
	return ""
}

// </PRIVATE METHODS>
////////////////////////////////////////////////////////////////////////////////////////////
