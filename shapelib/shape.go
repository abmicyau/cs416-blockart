package shapelib

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////////////////
// <OBJECT DEFINTIONS>

// TODO: Remove these errors (since they're already defined in blockartlib)
// and find some other way to notify for specific erros

// Empty
type OutOfBoundsError struct{}

func (e OutOfBoundsError) Error() string {
	return fmt.Sprintf("BlockArt: Shape is outside the bounds of the canvas")
}

// Contains the offending svg string.
type InvalidShapeSvgStringError string

func (e InvalidShapeSvgStringError) Error() string {
	return fmt.Sprintf("BlockArt: Bad shape svg string [%s]", string(e))
}

// Contains details
type InvalidShapeFillStrokeError string

func (e InvalidShapeFillStrokeError) Error() string {
	return fmt.Sprintf("BlockArt: ", string(e))
}

// Contains amount of ink remaining.
type InsufficientInkError uint32

func (e InsufficientInkError) Error() string {
	return fmt.Sprintf("BlockArt: Not enough ink to addShape [%d]", uint32(e))
}

// Contains the hash of the shape that this shape overlaps with.
type ShapeOverlapError string

func (e ShapeOverlapError) Error() string {
	return fmt.Sprintf("BlockArt: Shape overlaps with a previously added shape [%s]", string(e))
}

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota
)

// Represents a command with type(M, H, L, m, h, l, etc.)
// and specified (x, y) coordinate
type Command struct {
	CmdType string

	X int64
	Y int64
}

// Represents a point with (x, y) coordinate
type Point struct {
	X int64
	Y int64
}

func (p Point) inBound(xMax uint32, yMax uint32) bool {
	return p.X > 0 && p.Y > 0 && p.X < int64(xMax) && p.Y < int64(yMax)
}

type Shape struct {
	Owner string

	ShapeType      ShapeType
	ShapeSvgString string
	Fill           string
	Stroke         string
}

// Determines whether the shape is valid
func (s *Shape) IsValid(xMax uint32, yMax uint32) (valid bool, geometry ShapeGeometry, err error) {
	if geometry, err = s.GetGeometry(); err == nil {
		valid, err = geometry.isValid(xMax, yMax)
	}

	return
}

// Gets the shape geometry of a a provided shape
func (s *Shape) GetGeometry() (geometry ShapeGeometry, err error) {
	commands, err := getCommands(s.ShapeSvgString)
	if err != nil {
		return
	}

	geometry.ShapeSvgString, geometry.Fill = s.ShapeSvgString, s.Fill
	geometry.Min, geometry.Max, geometry.Vertices = Point{}, Point{}, make([]Point, len(commands))
	absPos, relPos := Point{0, 0}, Point{0, 0}
	for i := range commands {
		_command := commands[i]

		switch _command.CmdType {
		case "M":
			absPos.X, absPos.Y = _command.X, _command.Y
			relPos.X, relPos.Y = _command.X, _command.Y

			geometry.Vertices[i] = Point{relPos.X, relPos.Y}
		case "H":
			relPos.X = _command.X

			geometry.Vertices[i] = Point{relPos.X, absPos.Y}
		case "V":
			relPos.Y = _command.Y

			geometry.Vertices[i] = Point{absPos.X, relPos.Y}
		case "L":
			relPos.X, relPos.Y = _command.X, _command.Y

			geometry.Vertices[i] = Point{relPos.X, relPos.Y}
		case "h":
			relPos.X = relPos.X + _command.X

			geometry.Vertices[i] = Point{relPos.X, relPos.Y}
		case "v":
			relPos.Y = relPos.Y + _command.Y

			geometry.Vertices[i] = Point{relPos.X, relPos.Y}
		case "l":
			relPos.X, relPos.Y = relPos.X+_command.X, relPos.Y+_command.Y

			geometry.Vertices[i] = Point{relPos.X, relPos.Y}
		case "Z":
			geometry.Vertices[i] = geometry.Vertices[0]
		case "z":
			geometry.Vertices[i] = geometry.Vertices[0]
		}

		if i == 0 {
			geometry.Min = relPos
			geometry.Max = relPos
		} else {
			if relPos.X < geometry.Min.X {
				geometry.Min.X = relPos.X
			} else if relPos.X > geometry.Max.X {
				geometry.Max.X = relPos.X
			}

			if relPos.Y < geometry.Min.Y {
				geometry.Min.Y = relPos.Y
			} else if relPos.Y > geometry.Max.Y {
				geometry.Max.Y = relPos.Y
			}
		}
	}

	// Make sure its closed
	if s.Fill != "transparent" && geometry.Vertices[0] != geometry.Vertices[len(geometry.Vertices)-1] {
		err = InvalidShapeSvgStringError(s.ShapeSvgString)
		return
	}

	geometry.LineSegments = getLineSegments(geometry.Vertices)

	return
}

type ShapeGeometry struct {
	ShapeSvgString string
	Fill           string

	Vertices     []Point
	LineSegments []LineSegment
	Min          Point
	Max          Point
}

// Computes the ink required for the given shape according
// to the fill specification.
func (s *ShapeGeometry) GetInkCost() (inkUnits uint64) {
	if s.Fill == "transparent" {
		inkUnits = computePerimeter(s.LineSegments)
	} else {
		inkUnits = computePixelArea(s.Min, s.Max, s.LineSegments)
	}

	return
}

// Determines if the following conditions hold:
// - The shape is within the given bounding requirements
// - The shape is non-overlapping if not transparent
func (s *ShapeGeometry) isValid(xMax uint32, yMax uint32) (valid bool, err error) {
	valid = true

	for _, vertex := range s.Vertices {
		if valid = vertex.inBound(xMax, yMax); !valid {
			err = new(OutOfBoundsError)

			return
		}
	}

	if s.Fill != "transparent" {
		for i := range s.LineSegments {
			curSeg := s.LineSegments[i]

			for j := range s.LineSegments {
				if i != j && curSeg.Intersects(s.LineSegments[j]) == true {
					valid = false
					err = InvalidShapeSvgStringError(s.ShapeSvgString)

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

// Determines if a proposed shape overlape this shape.
func (s *ShapeGeometry) HasOverlap(_s ShapeGeometry) bool {
	// Easy preliminary: does the bounding box of _s encompass s
	if _s.Fill != "transparent" && (_s.Min.X <= s.Min.X && _s.Min.Y <= s.Min.Y) && (_s.Max.X >= s.Max.X && _s.Max.Y >= s.Max.Y) {
		return true
	}

	if intersectExists(s.LineSegments, _s.LineSegments) {
		return true
	} else if s.Fill != "transparent" && containsVertex(s.Min, s.Max, s.LineSegments, _s.Vertices) {
		return true
	} else {
		return false
	}
}

// Represents a line segment with start and end points
// and implicit equation format ax + by = c
type LineSegment struct {
	Start Point
	End   Point

	A int64
	B int64
	C int64
}

// Determines the length of a given line segments
// rounding to the nearest integer greater than the float
func (l LineSegment) Length() uint64 {
	if l.Start == l.End {
		return 1
	} else {
		a, b := float64(l.Start.X-l.End.X), float64(l.Start.Y-l.End.Y)
		c := math.Sqrt(math.Pow(a, 2) + math.Pow(b, 2))

		return uint64(math.Ceil(c))
	}
}

// Determines if a point lies on a line segment
func (l LineSegment) HasPoint(p Point) bool {
	x1, y1, x2, y2 := l.Start.X, l.Start.Y, l.End.X, l.End.Y

	return ((y1 <= p.Y && p.Y <= y2) || (y1 >= p.Y && p.Y >= y2)) &&
		((x1 <= p.X && p.X <= x2) || (x1 >= p.X && p.X >= x2))
}

// Determines if two line segments are parallel
func (l LineSegment) IsColinear(_l LineSegment) bool {
	a1, b1, c1 := l.A, l.B, l.C
	a2, b2, c2 := _l.A, _l.B, _l.C

	if a1 == a2 && b1 == b2 && c1 == c2 {
		return true
	} else if a1 == -1*a2 && b1 == -1*b2 && c1 == -1*c2 {
		return true
	} else {
		return false
	}
}

func (l LineSegment) HasColinearIntersect(_l LineSegment) bool {
	a1, b1 := l.A, l.B
	a2, b2 := _l.A, _l.B

	det := a1*b2 - a2*b1
	if det == 0 && (l.HasPoint(_l.Start) || l.HasPoint(_l.End)) {
		return true
	} else {
		return false
	}
}

func (l LineSegment) GetIntersect(_l LineSegment) (point Point, err error) {
	var x, y int64

	a1, b1, c1 := l.A, l.B, l.C
	a2, b2, c2 := _l.A, _l.B, _l.C

	det := a1*b2 - a2*b1
	if det == 0 {
		if l.HasPoint(_l.Start) || l.HasPoint(_l.End) {
			err = errors.New("Lines are colinear.")
		} else {
			err = errors.New("Lines are parallel but not colinear.")
		}

		return
	} else {
		x = (b2*c1 - b1*c2) / det
		y = (a1*c2 - a2*c1) / det
	}

	p := Point{x, y}
	if l.HasPoint(p) && _l.HasPoint(p) {
		point = p
	} else {
		err = errors.New("No intersect exists.")
	}

	return
}

// Determines if two line segment intersect within
// their given start and end points
func (l LineSegment) Intersects(_l LineSegment) bool {
	colinear := l.IsColinear(_l)
	if colinear && l.HasColinearIntersect(_l) {
		return true
	} else if _, err := l.GetIntersect(_l); err == nil {
		return true
	} else {
		return false
	}
}

// </OBJECT DEFINTIONS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <FUNCTIONS>

// Normalizes SVG string removing all spaces and adding commas
func normalizeSvgString(svg string) (normSvg string) {
	// Set commas between numbers
	re := regexp.MustCompile("(-?\\d+)((\\s+|\\s?),(\\s+|\\s?)|(\\s+))(-?\\d+)")
	normSvg = re.ReplaceAllString(svg, "$1,$6")

	// Remove space between command and number
	re = regexp.MustCompile("(\\s+|\\s?)([a-zA-Z])(\\s+|\\s?)")
	normSvg = re.ReplaceAllString(normSvg, "$2")

	return
}

// Extracts commands from provided SVG string
func getCommands(svg string) (commands []Command, err error) {
	var _commands []Command

	normSvg := normalizeSvgString(svg)
	for {
		_command := Command{}

		re := regexp.MustCompile("(^.+?)([a-zA-Z])(.*)")
		cmdString := strings.Trim(re.ReplaceAllString(normSvg, "$1"), " ")

		pos := strings.Split(string(cmdString[1:]), ",")
		posEmpty := len(pos) <= 1 && pos[0] == ""

		cmdType := string(cmdString[0])
		switch cmdType {
		case "M":
			_command.CmdType = "M"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.X, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "H":
			_command.CmdType = "H"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.X, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "V":
			_command.CmdType = "V"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.Y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "L":
			_command.CmdType = "L"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.X, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "h":
			_command.CmdType = "h"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.X, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "v":
			_command.CmdType = "v"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.Y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "l":
			_command.CmdType = "l"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.X, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		}

		if err != nil {
			break
		}

		if _command.CmdType != "" {
			_commands = append(_commands, _command)
		}

		normSvg = strings.Replace(normSvg, cmdString, "", 1)
		normSvg = strings.Trim(normSvg, " ")
		if normSvg == "" {
			break
		}
	}

	if err == nil {
		commands = _commands
	}

	return
}

// Determines if the given vertex exists in a set of vertices
func vertexExists(v Point, vertices []Point) bool {
	for _, vertex := range vertices {
		if v.X == vertex.X && v.Y == vertex.Y {
			return true
		}
	}

	return false
}

// Determines if a line segment exists in a set of line segments
func segmentExists(lineSegment LineSegment, lineSegments []LineSegment) bool {
	for _, _lineSegment := range lineSegments {
		if lineSegment.Start == _lineSegment.Start && lineSegment.End == _lineSegment.End {
			return true
		} else if lineSegment.Start == _lineSegment.End && lineSegment.End == _lineSegment.Start {
			return true
		}
	}

	return false
}

// Extracts line segment from 2 vertices
func getLineSegment(v1 Point, v2 Point) (lineSegment LineSegment) {
	lineSegment.Start = v1
	lineSegment.End = v2

	lineSegment.A = v2.Y - v1.Y
	lineSegment.B = v1.X - v2.X
	lineSegment.C = lineSegment.A*v1.X + lineSegment.B*v1.Y

	return
}

// Determines if an intersect exists between two sets of line segments
func intersectExists(lineSegments []LineSegment, _lineSegments []LineSegment) bool {
	for _, _lineSegment := range _lineSegments {
		for _, lineSegment := range lineSegments {
			if intersect := lineSegment.Intersects(_lineSegment); intersect {
				return true
			}
		}
	}

	return false
}

/* Given a set of polygon intersects and vertex intersects, where the polygon
intersects belong to some polygon and vertex intersects being vertices of
some test shape, if the following is true:

An ordered configuration of one vertex intersect and all polygon intersections
exists where this is an odd number of polygon intersects on either side of then
one vertex intersect.

For example (p = polygon intersect and v = vertex intersect):
	ppp v ppppp

If this is true, the test shape is WITHIN the polygon.
*/
func hasOddConfiguration(polyIntersects []Point, vertexIntersects []Point) bool {
	for _, v := range vertexIntersects {
		var leftIntersects uint32
		var rightIntersects uint32

		for _, p := range polyIntersects {
			if p.X < v.X {
				leftIntersects++
			} else {
				rightIntersects++
			}
		}

		if (leftIntersects%2 != 0) && (rightIntersects%2 != 0) {
			return true
		}
	}

	return false
}

// Extracts line segments (in order) from provided vertices,
// where each vertex is connected to the next vertex
func getLineSegments(vertices []Point) (lineSegments []LineSegment) {
	for i := 0; i < len(vertices)-2; i++ {
		var v1, v2 Point
		var lineSegment LineSegment

		v1 = vertices[i]
		v2 = vertices[i+1]

		lineSegment.Start = v1
		lineSegment.End = v2

		lineSegment.A = v2.Y - v1.Y
		lineSegment.B = v1.X - v2.X
		lineSegment.C = lineSegment.A*v1.X + lineSegment.B*v1.Y

		lineSegments = append(lineSegments, lineSegment)
	}

	return
}

// Determines if any of the vertices are contained with a polygon, using a scanline.
func containsVertex(min Point, max Point, lineSegments []LineSegment, vertices []Point) bool {
	for y := min.Y; y <= max.Y; y++ {
		var polyIntersects []Point
		var vertexIntersects []Point

		scanLine := getLineSegment(Point{min.X, y}, Point{max.X, y})

		// Get all polygon intersects on this scanline
		for _, l := range lineSegments {
			if scanLine.IsColinear(l) {
				polyIntersects = append(polyIntersects, l.Start, l.End)
			} else {
				hasIntersect := l.Intersects(scanLine)
				intersect, err := l.GetIntersect(scanLine)
				if hasIntersect && err == nil && !vertexExists(intersect, polyIntersects) {
					polyIntersects = append(polyIntersects, intersect)
				}
			}
		}

		// Get all vertex intersects on this scanline
		for _, v := range vertices {
			if scanLine.HasPoint(v) {
				vertexIntersects = append(vertexIntersects, v)
			}
		}

		if len(vertexIntersects) > 0 && hasOddConfiguration(polyIntersects, vertexIntersects) {
			return true
		}
	}

	return false
}

// Computes the total length of all segments
func computePerimeter(lineSegments []LineSegment) (perimeter uint64) {
	var computedSegments []LineSegment

	for _, lineSegment := range lineSegments {
		if !segmentExists(lineSegment, computedSegments) {
			computedSegments = append(computedSegments, lineSegment)

			perimeter = perimeter + lineSegment.Length()
		}
	}

	return
}

// Computes the regular geometric area of polygon
// NOTE: This computes the 'geometric' area, but which doesnt match the actual pixel-based area
func computeGeoArea(vertices []Point) uint64 {
	var area int64
	for i, v1 := range vertices {
		var v2 Point
		if i == len(vertices)-1 {
			v2 = vertices[0]
		} else {
			v2 = vertices[i+1]
		}

		area = area + (v1.X*v2.Y - v2.X*v1.Y)
	}

	return uint64(area / 2)
}

// Computes the total area within a polygon using a scanline
// descending down the y-axis
// NOTE: This computes the actual number of pixels required to draw shape
// Doesn't exlude the actual line segments
func computePixelArea(min Point, max Point, lineSegments []LineSegment) (area uint64) {
	for y := min.Y; y <= max.Y; y++ {
		var intersects []Point

		scanLine := getLineSegment(Point{min.X, y}, Point{max.X, y})

		// Check intersections with all line segments
		for _, l := range lineSegments {
			if scanLine.IsColinear(l) { // If parallel, extract the start and end points
				intersects = append(intersects, l.Start, l.End)
			} else { // Get intersection
				hasIntersect := l.Intersects(scanLine)
				if intersect, err := l.GetIntersect(scanLine); hasIntersect && err == nil {
					intersects = append(intersects, intersect)
				}
			}
		}

		/*
			Compute the lengths for all line segments generated by intersects on scanline.

			Example of cases (where the letters are intersects/vertices):
			*Joint + non-vertices*
				ABBC 		 -> AB BC 				 [Edge then joint then edge]
				{B is a shared vertex}

			*Parallel path and non-vertices*
				ABBCCDDA -> AB BC CD DA    [Rectangle]
				{A B C D are shared vertices}

				AABBC 	 -> AB BC 				 [Parallel line then edge]
				{A B are shared vertices}

			*Non-vertices*
				ABCDEF 	 -> AB CD	EF			 [Any polygon where scanline not on vertices]
		*/
		if len(intersects) > 1 {
			var computedSegments []LineSegment

			i := 0
			for {
				lineSegment := getLineSegment(intersects[i], intersects[i+1])

				if lineSegment.Start == lineSegment.End { // If both vertices are same point, incremement by one
					i = i + 1
				} else if segmentExists(lineSegment, computedSegments) { // If we already calculated this segment, skip
					i = i + 2
				} else { // Otherwise, we have a valid segment, add length to area
					computedSegments = append(computedSegments, lineSegment)

					area = area + lineSegment.Length()
					i = i + 2
				}

				if len(intersects) <= (i + 1) {
					break
				}
			}
		}
	}

	return
}

// </FUNCTIONS>
////////////////////////////////////////////////////////////////////////////////////////////
