package blockartlib

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

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
	a, b := float64(l.start.x-l.end.x), float64(l.start.y-l.end.y)
	c := math.Sqrt(math.Pow(a, 2) + math.Pow(b, 2))

	return uint64(math.Ceil(c))
}

// Determines if a point lies on a line segment
func (l LineSegment) hasPoint(p Point) bool {
	x1, y1, x2, y2 := l.start.x, l.start.y, l.end.x, l.end.y

	return ((y1 <= p.y && p.y <= y2) || (y1 >= p.y && p.y >= y2)) &&
		((x1 <= p.x && p.x <= x2) || (x1 >= p.x && p.x >= x2))
}

// Determines if two line segment intersect within
// their given start and end points
func (l LineSegment) intersects(_l LineSegment) bool {
	var x, y int64

	a1, b1, c1 := l.a, l.b, l.c
	a2, b2, c2 := _l.a, _l.b, _l.c

	det := a1*b2 - a2*b1
	if det == 0 {
		if l.hasPoint(_l.start) || l.hasPoint(_l.end) {
			return true
		} else {
			return false
		}
	} else {
		x = (b2*c1 - b1*c2) / det
		y = (a1*c2 - a2*c1) / det
	}

	p := Point{x, y}
	if l.hasPoint(p) && _l.hasPoint(p) {
		return true
	} else {
		return false
	}
}

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

		normSvg = strings.TrimLeft(normSvg, cmdString)
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
