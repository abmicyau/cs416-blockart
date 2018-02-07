package blockartlib

import (
	"regexp"
	"strconv"
	"strings"
)

// Represents a command with type(M, H, L, m, h, l, etc.)
type Command struct {
	cmdType string

	x int64
	y int64
}

// Represents a point
type Point struct {
	x int64
	y int64
}

// Represents a line segment with start and end point
// with equation format ax + by = c
type LineSegment struct {
	start Point
	end   Point

	a int64
	b int64
	c int64
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

// Extracts line segments (in order) from provided vertices
func getLineSegments(vertices []Point) (lineSegments []LineSegment) {
	for i := range vertices {
		var v1 Point
		var v2 Point
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

// Determines if two lines intersect within their given start and end points
func linesOverlap(existLine LineSegment, newLine LineSegment) bool {
	var x int64
	var y int64

	a1 := existLine.a
	b1 := existLine.b
	c1 := existLine.c

	a2 := newLine.a
	b2 := newLine.b
	c2 := newLine.c

	det := a1*b2 - a2*b1
	if det == 0 {
		if isBetween(newLine.start, existLine.start, existLine.end) || isBetween(newLine.end, existLine.start, existLine.end) {
			return true
		} else {
			return false
		}
	} else {
		x = (b2*c1 - b1*c2) / det
		y = (a1*c2 - a2*c1) / det
	}

	if isBetween(Point{x, y}, existLine.start, existLine.end) &&
		isBetween(Point{x, y}, newLine.start, newLine.end) {
		return true
	} else {
		return false
	}
}

// Determines if a point lies on the line between start and end points
func isBetween(p Point, start Point, end Point) bool {
	return ((start.y <= p.y && p.y <= end.y) || (start.y >= p.y && p.y >= end.y)) &&
		((start.x <= p.x && p.x <= end.x) || (start.x >= p.x && p.x >= end.x))
}
