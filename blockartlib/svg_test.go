package blockartlib

import (
	"strconv"
	"testing"
)

// Test normalization
func TestNormalizeSvgString(t *testing.T) {
	svgString := "   M 10 10 L 5 , 5 h -3 Z"
	svgNorm := normalizeSvgString(svgString)
	svgExpected := "M10,10L5,5h-3Z"

	if svgNorm != svgExpected {
		t.Error("Expected "+svgExpected+", got ", svgNorm)
	}
}

// Test command parsing
func TestGetCommands(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.evaluateSvgString()

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

// Test vertices generated from commands
func TestGetVertices(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.evaluateSvgString()

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

// Test line segments generated from vertices
func TestGetLineSegments(t *testing.T) {
	shape := Shape{shapeSvgString: "   M 10 10 L 5 , 5 h -3 Z"}
	shape.evaluateSvgString()

	lineSegments := getLineSegments(shape.vertices)
	lineSegmentsExpected := []LineSegment{
		LineSegment{a: -5, b: 5, c: 0},
		LineSegment{a: 0, b: 3, c: 15},
		LineSegment{a: 5, b: -8, c: -30}}

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

// Test line-to-line overlap
func TestLineOverlap(t *testing.T) {
	shape1 := Shape{shapeSvgString: "M 10 10 L 5 5 "}
	shape2 := Shape{shapeSvgString: "M 5 5 L 10 10"}
	shape3 := Shape{shapeSvgString: "M 7 5 L 5 10 v -2 Z"}
	shape1.evaluateSvgString()
	shape2.evaluateSvgString()
	shape3.evaluateSvgString()

	lineSegments1 := getLineSegments(shape1.vertices)
	lineSegments2 := getLineSegments(shape2.vertices)
	lineSegments3 := getLineSegments(shape3.vertices)

	// Test parallel lines
	if linesOverlap(lineSegments1[0], lineSegments2[0]) != true {
		t.Error("Expected true, got false")
	}

	if linesOverlap(lineSegments1[0], lineSegments2[1]) != true {
		t.Error("Expected true, got false")
	}

	// Test non-parallel lines
	if linesOverlap(lineSegments1[0], lineSegments3[0]) != true {
		t.Error("Expected true, got false")
	}

	if linesOverlap(lineSegments1[0], lineSegments3[2]) != true {
		t.Error("Expected true, got false")
	}

	if linesOverlap(lineSegments1[0], lineSegments3[1]) != false {
		t.Error("Expected false, got true")
	}
}

// Test shape validity
func TestShapeValid(t *testing.T) {
	xMax := uint32(100)
	yMax := uint32(100)

	shapeLineInBound := Shape{fill: "transparent", shapeSvgString: "M 10 10 L 5 5 "}
	shapeOutOfMinBound := Shape{fill: "transparent", shapeSvgString: "M 5 5 h -7"}
	shapeOutOfMaxBound := Shape{fill: "transparent", shapeSvgString: "M 7 5 h 10000000"}
	shapeSelfIntersectTrans := Shape{fill: "transparent", shapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}
	shapeSelfIntersectNonTrans := Shape{fill: "non-transparent", shapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}
	shapeLineInBound.evaluateSvgString()
	shapeOutOfMinBound.evaluateSvgString()
	shapeOutOfMaxBound.evaluateSvgString()
	shapeSelfIntersectTrans.evaluateSvgString()
	shapeSelfIntersectNonTrans.evaluateSvgString()

	if valid, err := shapeLineInBound.isValid(xMax, yMax); valid != true {
		t.Error("Expected valid shape, got", err)
	}

	if valid, err := shapeSelfIntersectTrans.isValid(xMax, yMax); valid != true {
		t.Error("Expected valid shape, got", err)
	}

	if valid, err := shapeOutOfMinBound.isValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}

	if valid, err := shapeOutOfMaxBound.isValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}

	if valid, err := shapeSelfIntersectNonTrans.isValid(xMax, yMax); valid != false || err == nil {
		t.Error("Expected invalid shape, got valid")
	}
}

// Test ink usage
func TestInkUsage(t *testing.T) {
	shape1 := Shape{fill: "transparent", shapeSvgString: "M 10 10 L 5 5 "}
	shape2 := Shape{fill: "transparent", shapeSvgString: "M 5 5 L 10 10 h -5 L 10 5 Z"}
	shape1.evaluateSvgString()
	shape2.evaluateSvgString()

	if ink := shape1.computeInkUsage(); ink != 16 {
		t.Error("Expected 16 ink units, got", strconv.FormatUint(ink, 10))
	}

	if ink := shape2.computeInkUsage(); ink != 26 {
		t.Error("Expected 14 ink units, got", strconv.FormatUint(ink, 10))
	}
}
