package errorLib

import (
	"fmt"
	"reflect"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFS>

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

// Contains details
type InvalidShapeFillStrokeError string

func (e InvalidShapeFillStrokeError) Error() string {
	return fmt.Sprintf("BlockArt: ", string(e))
}

// Empty
type InvalidSignatureError struct{}

func (e InvalidSignatureError) Error() string {
	return fmt.Sprintf("Invalid signature.", nil)
}

// Contains the token
type InvalidTokenError string

func (e InvalidTokenError) Error() string {
	return fmt.Sprintf("Invalid token: ", string(e))
}

type ValidationError string

func (e ValidationError) Error() string {
	return fmt.Sprintf("Problem occured with validation on", string(e))
}

// </ERROR DEFS>
////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////
// <FUNCTIONS>

func IsType(err error, errType string) bool {
	return strings.HasSuffix(reflect.TypeOf(err).String(), errType)
}
