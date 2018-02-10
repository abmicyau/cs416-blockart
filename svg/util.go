package svg

import (
	"fmt"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"crypto/ecdsa"
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

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota
)

type Shape struct {
	ShapeType      ShapeType
	ShapeSvgString string
	Fill           string
	Stroke         string
	Owner          ecdsa.PublicKey

    Commands       []Command
    Vertices       []Point
    LineSegments   []LineSegment
    Min            Point
    Max            Point
}

// Represents a command with type(M, H, L, m, h, l, etc.)
// and specified (x, y) coordinate
type Command struct {
	cmdType string

	x int64
	y int64
}

// Represents a point with (x, y) coordinate
type Point struct {
	x int64
	y int64
}

// Computes the ink required for the given shape according
// to the fill specification.
func (s *Shape) computeInkRequired() (inkUnits uint64) {
    if s.Fill == "transparent" {
        inkUnits = computePerimeter(s.LineSegments)
    } else {
        inkUnits = computePixelArea(s.Min, s.Max, s.LineSegments)
    }

    return
}

// Determines if, within a canvas bound, a proposed shape is valid.
func (s *Shape) isValid(xMax uint32, yMax uint32) (valid bool, err error) {
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
                if i != j && curSeg.intersects(s.LineSegments[j]) == true {
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

/* Evaluates a shapes provided SVG string to determine the following:
- Its min and max bounding box
- Its parsed commands
- Its vertices
- Its line segments
*/
func (s *Shape) evaluateSvgString() (err error) {
    s.Min, s.Max = Point{}, Point{}
    if s.Commands, err = getCommands(s.ShapeSvgString); err != nil {
        return
    }

    vertices := make([]Point, len(s.Commands))

    absPos, relPos := Point{0, 0}, Point{0, 0}
    for i := range s.Commands {
        _command := s.Commands[i]

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

        if i == 0 {
            s.Min = relPos
            s.Max = relPos
        } else {
            if relPos.x < s.Min.x {
                s.Min.x = relPos.x
            } else if relPos.x > s.Max.x {
                s.Max.x = relPos.x
            }

            if relPos.y < s.Min.y {
                s.Min.y = relPos.y
            } else if relPos.y > s.Max.y {
                s.Max.y = relPos.y
            }
        }
    }

    s.Vertices = vertices
    s.LineSegments = getLineSegments(vertices)

    return
}

// Determines if a proposed shape overlape this shape.
func (s *Shape) hasOverlap(_s Shape) bool {
    // Easy preliminary: does the bounding box of _s encompass s
    if _s.Fill != "transparent" && (_s.Min.x <= s.Min.x && _s.Min.y <= s.Min.y) && (_s.Max.x >= s.Max.x && _s.Max.y >= s.Max.y) {
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

func (p Point) inBound(xMax uint32, yMax uint32) bool {
	return p.x > 0 && p.y > 0 && p.x < int64(xMax) && p.y < int64(yMax)
}

// Represents a line segment with start and end points
// and implicit equation format ax + by = c
type LineSegment struct {
	start Point
	end   Point

	a int64
	b int64
	c int64
}

// Determines the length of a given line segments
// rounding to the nearest integer greater than the float
func (l LineSegment) length() uint64 {
	if l.start == l.end {
		return 1
	} else {
		a, b := float64(l.start.x-l.end.x), float64(l.start.y-l.end.y)
		c := math.Sqrt(math.Pow(a, 2) + math.Pow(b, 2))

		return uint64(math.Ceil(c))
	}
}

// Determines if a point lies on a line segment
func (l LineSegment) hasPoint(p Point) bool {
	x1, y1, x2, y2 := l.start.x, l.start.y, l.end.x, l.end.y

	return ((y1 <= p.y && p.y <= y2) || (y1 >= p.y && p.y >= y2)) &&
		((x1 <= p.x && p.x <= x2) || (x1 >= p.x && p.x >= x2))
}

// Determines if two line segments are parallel
func (l LineSegment) isColinear(_l LineSegment) bool {
	a1, b1, c1 := l.a, l.b, l.c
	a2, b2, c2 := _l.a, _l.b, _l.c

	if a1 == a2 && b1 == b2 && c1 == c2 {
		return true
	} else if a1 == -1*a2 && b1 == -1*b2 && c1 == -1*c2 {
		return true
	} else {
		return false
	}
}

func (l LineSegment) hasColinearIntersect(_l LineSegment) bool {
	a1, b1 := l.a, l.b
	a2, b2 := _l.a, _l.b

	det := a1*b2 - a2*b1
	if det == 0 && (l.hasPoint(_l.start) || l.hasPoint(_l.end)) {
		return true
	} else {
		return false
	}
}

func (l LineSegment) getIntersect(_l LineSegment) (point Point, err error) {
	var x, y int64

	a1, b1, c1 := l.a, l.b, l.c
	a2, b2, c2 := _l.a, _l.b, _l.c

	det := a1*b2 - a2*b1
	if det == 0 {
		if l.hasPoint(_l.start) || l.hasPoint(_l.end) {
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
	if l.hasPoint(p) && _l.hasPoint(p) {
		point = p
	} else {
		err = errors.New("No intersect exists.")
	}

	return
}

// Determines if two line segment intersect within
// their given start and end points
func (l LineSegment) intersects(_l LineSegment) bool {
	colinear := l.isColinear(_l)
	if colinear && l.hasColinearIntersect(_l) {
		return true
	} else if _, err := l.getIntersect(_l); err == nil {
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
			_command.cmdType = "M"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.x, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "H":
			_command.cmdType = "H"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.x, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "V":
			_command.cmdType = "V"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "L":
			_command.cmdType = "L"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.x, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "h":
			_command.cmdType = "h"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.x, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "v":
			_command.cmdType = "v"

			if posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "l":
			_command.cmdType = "l"

			if len(pos) < 2 || posEmpty {
				return nil, InvalidShapeSvgStringError(svg)
			} else {
				_command.x, _ = strconv.ParseInt(pos[0], 10, 64)
				_command.y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		}

		if err != nil {
			break
		}

		if _command.cmdType != "" {
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
		if v.x == vertex.x && v.y == vertex.y {
			return true
		}
	}

	return false
}

// Determines if a line segment exists in a set of line segments
func segmentExists(lineSegment LineSegment, lineSegments []LineSegment) bool {
	for _, _lineSegment := range lineSegments {
		if lineSegment.start == _lineSegment.start && lineSegment.end == _lineSegment.end {
			return true
		} else if lineSegment.start == _lineSegment.end && lineSegment.end == _lineSegment.start {
			return true
		}
	}

	return false
}

// Extracts line segment from 2 vertices
func getLineSegment(v1 Point, v2 Point) (lineSegment LineSegment) {
	lineSegment.start = v1
	lineSegment.end = v2

	lineSegment.a = v2.y - v1.y
	lineSegment.b = v1.x - v2.x
	lineSegment.c = lineSegment.a*v1.x + lineSegment.b*v1.y

	return
}

// Determines if an intersect exists between two sets of line segments
func intersectExists(lineSegments []LineSegment, _lineSegments []LineSegment) bool {
	for _, _lineSegment := range _lineSegments {
		for _, lineSegment := range lineSegments {
			if intersect := lineSegment.intersects(_lineSegment); intersect {
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
			if p.x < v.x {
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
	for i := range vertices {
		var v1, v2 Point
		var lineSegment LineSegment

		v1 = vertices[i]
		if i == len(vertices)-1 {
			v2 = vertices[0]
		} else {
			v2 = vertices[i+1]
		}

		lineSegment.start = v1
		lineSegment.end = v2

		lineSegment.a = v2.y - v1.y
		lineSegment.b = v1.x - v2.x
		lineSegment.c = lineSegment.a*v1.x + lineSegment.b*v1.y

		lineSegments = append(lineSegments, lineSegment)
	}

	return
}

// Determines if any of the vertices are contained with a polygon, using a scanline.
func containsVertex(min Point, max Point, lineSegments []LineSegment, vertices []Point) bool {
	for y := min.y; y <= max.y; y++ {
		var polyIntersects []Point
		var vertexIntersects []Point

		scanLine := getLineSegment(Point{min.x, y}, Point{max.x, y})

		// Get all polygon intersects on this scanline
		for _, l := range lineSegments {
			if scanLine.isColinear(l) {
				polyIntersects = append(polyIntersects, l.start, l.end)
			} else {
				hasIntersect := l.intersects(scanLine)
				intersect, err := l.getIntersect(scanLine)
				if hasIntersect && err == nil && !vertexExists(intersect, polyIntersects) {
					polyIntersects = append(polyIntersects, intersect)
				}
			}
		}

		// Get all vertex intersects on this scanline
		for _, v := range vertices {
			if scanLine.hasPoint(v) {
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

			perimeter = perimeter + lineSegment.length()
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

		area = area + (v1.x*v2.y - v2.x*v1.y)
	}

	return uint64(area / 2)
}

// Computes the total area within a polygon using a scanline
// descending down the y-axis
// NOTE: This computes the actual number of pixels required to draw shape
// Doesn't exlude the actual line segments
func computePixelArea(min Point, max Point, lineSegments []LineSegment) (area uint64) {
	for y := min.y; y <= max.y; y++ {
		var intersects []Point

		scanLine := getLineSegment(Point{min.x, y}, Point{max.x, y})

		// Check intersections with all line segments
		for _, l := range lineSegments {
			if scanLine.isColinear(l) { // If parallel, extract the start and end points
				intersects = append(intersects, l.start, l.end)
			} else { // Get intersection
				hasIntersect := l.intersects(scanLine)
				if intersect, err := l.getIntersect(scanLine); hasIntersect && err == nil {
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

				if lineSegment.start == lineSegment.end { // If both vertices are same point, incremement by one
					i = i + 1
				} else if segmentExists(lineSegment, computedSegments) { // If we already calculated this segment, skip
					i = i + 2
				} else { // Otherwise, we have a valid segment, add length to area
					computedSegments = append(computedSegments, lineSegment)

					area = area + lineSegment.length()
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
