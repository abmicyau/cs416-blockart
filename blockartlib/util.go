package blockartlib

import (
	"regexp"
	"strconv"
	"strings"
)

type Command struct {
	cmdType string

	x int64
	y int64
}

type Point struct {
	x uint32
	y uint32
}

type LineSegment struct {
	a int32
	b int32
	c int32
}

func normalizeSvgString(svg string) (normSvg string) {
	// Set commas between numbers
	re := regexp.MustCompile("(-?\\d+)((\\s+|\\s?),(\\s+|\\s?)|(\\s+))(-?\\d+)")
	normSvg = re.ReplaceAllString(svg, "$1,$6")

	// Remove space between command and number
	re = regexp.MustCompile("(\\s+|\\s?)([a-zA-Z])(\\s+|\\s?)")
	normSvg = re.ReplaceAllString(normSvg, "$2")

	return
}

func getCommands(svg string) (commands []Command) {
	normSvg := normalizeSvgString(svg)

	i := 0
	for {
		if len(normSvg) <= 1 {
			break
		}
		_command := Command{}

		re := regexp.MustCompile("(^.+?)([a-zA-Z])(.*)")
		cmdString := re.ReplaceAllString(normSvg, "$1")

		cmd := string(cmdString[0])
		_command.cmdType = cmd

		if len(cmdString) > 1 {
			pos := strings.Split(string(cmdString[1:]), ",")
			if len(pos) >= 1 && pos[0] != "" {
				_x, _ := strconv.ParseInt(pos[0], 10, 64)
				_command.x = _x
			}

			if len(pos) > 1 && pos[1] != "" {
				_y, _ := strconv.ParseInt(pos[1], 10, 64)
				_command.y = _y
			}
		}

		commands = append(commands, _command)

		normSvg = re.ReplaceAllString(normSvg, "$2$3")
		i++
	}

	return
}

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

		lineSegment.a = int32(v2.y - v1.y)
		lineSegment.b = -1 * int32(v2.x-v1.x)
		lineSegment.c = -1 * int32(lineSegment.a*int32(v1.x)+lineSegment.b*int32(v1.y))

		lineSegments = append(lineSegments, lineSegment)
	}

	return
}
