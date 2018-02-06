package blockartlib

import (
	"strconv"
	"testing"
)

func TestNormalizeSvgString(t *testing.T) {
	svgString := "   M 10 10 L 5 , 5 h -3 Z"
	svgNorm := normalizeSvgString(svgString)
	svgExpected := "M10,10L5,5h-3Z"

	if svgNorm != svgExpected {
		t.Error("Expected "+svgExpected+", got ", svgNorm)
	}
}

func TestGetCommands(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.ParseSvgString()

	commands := shape.commands
	commandsExpected := []Command{
		Command{"M", 10, 10},
		Command{"L", 5, 5},
		Command{"h", -3, 0},
		Command{"Z", 0, 0}}

	for i := range commands {
		svgCommand := commands[i]
		commandExpected := commandsExpected[i]

		if svgCommand.cmdType != commandExpected.cmdType {
			t.Error("Expected "+commandExpected.cmdType+", got ", svgCommand.cmdType)
		}

		if svgCommand.x != commandExpected.x {
			t.Error("Expected "+strconv.Itoa(int(commandExpected.x))+", got ", strconv.Itoa(int(svgCommand.x)))
		}

		if svgCommand.y != commandExpected.y {
			t.Error("Expected "+strconv.Itoa(int(commandExpected.y))+", got ", strconv.Itoa(int(svgCommand.y)))
		}
	}
}

func TestGetVertices(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.ParseSvgString()

	vertices := shape.vertices
	verticesExpected := []Point{
		Point{10, 10},
		Point{5, 5},
		Point{2, 5}}

	for i := range vertices {
		vertex := vertices[i]
		vertexExpected := verticesExpected[i]

		if vertex.x != vertexExpected.x {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.x))+", got ", strconv.Itoa(int(vertex.x)))
		}

		if vertex.y != vertexExpected.y {
			t.Error("Expected "+strconv.Itoa(int(vertexExpected.y))+", got ", strconv.Itoa(int(vertex.y)))
		}
	}
}

func TestGetLineSegments(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.ParseSvgString()

	lineSegments := getLineSegments(shape.vertices)
	lineSegmentsExpected := []LineSegment{
		LineSegment{-5, 5, 0},
		LineSegment{0, 3, -15},
		LineSegment{5, -8, 30}}

	for i := range lineSegments {
		lineSegment := lineSegments[i]
		lineSegmentExpected := lineSegmentsExpected[i]

		if lineSegment.a != lineSegmentExpected.a {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.a))+", got ", strconv.Itoa(int(lineSegment.a)))
		}

		if lineSegment.b != lineSegmentExpected.b {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.b))+", got ", strconv.Itoa(int(lineSegment.b)))
		}

		if lineSegment.c != lineSegmentExpected.c {
			t.Error("Expected "+strconv.Itoa(int(lineSegmentExpected.c))+", got ", strconv.Itoa(int(lineSegment.c)))
		}
	}
}
