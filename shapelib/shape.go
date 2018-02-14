package shapelib

import (
	"errors"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	//. "../errorlib"
	. "github.com/alfaeddie/proj1_b0z8_b4n0b_i5n8_m9r8/errorlib"
)

////////////////////////////////////////////////////////////////////////////////////////////
// <COMMAND>

// Represents a path command with type(M, H, L, m, h, l, etc.)
// and specified (x, y) coordinate
type PathCommand struct {
	CmdType string

	X int64
	Y int64
}

// Represents a circle command with type(X, Y, R, x, y, r) and value
type CircleCommand struct {
	CmdType string

	Val int64
}

// </COMMAND>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <SHAPE>

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	PATH ShapeType = iota
	CIRCLE
)

type Shape struct {
	Owner string

	ShapeType      ShapeType
	ShapeSvgString string
	Fill           string
	Stroke         string
}

func (s Shape) isPath() bool {
	return s.ShapeType == PATH
}

func (s Shape) isCircle() bool {
	return s.ShapeType == CIRCLE
}

// Determines whether the shape is valid
func (s Shape) IsValid(xMax uint32, yMax uint32) (valid bool, geometry ShapeGeometry, err error) {
	if s.Stroke == "" {
		err = InvalidShapeFillStrokeError("Shape stroke must be specified")
		return
	} else if s.Fill == "" {
		err = InvalidShapeFillStrokeError("Shape fill must be specified")
		return
	} else if s.Stroke == "transparent" && s.Fill == "transparent" {
		err = InvalidShapeFillStrokeError("Both fill and stroke cannot be transparent")
		return
	}

	if s.ShapeType == PATH {
		geometry, err = s.getPathGeometry()
	} else {
		geometry, err = s.getCircleGeometry()
	}

	if err == nil {
		valid, err = geometry.isValid(xMax, yMax)
	} else {
		return
	}

	return
}

func (s Shape) getCircleCommands() (commands []CircleCommand, err error) {
	normSvg := normalizeSvgString(s.ShapeSvgString)
	for {
		command := CircleCommand{}

		re := regexp.MustCompile("(^.+?)([a-zA-Z])(.*)")
		cmdString := strings.Trim(re.ReplaceAllString(normSvg, "$1"), " ")

		val, _ := strconv.Atoi(string(cmdString[1:]))
		cmdType := string(cmdString[0])
		switch cmdType {
		case "X", "x":
			command.CmdType = cmdType
			command.Val = int64(val)
		case "Y", "y":
			command.CmdType = cmdType
			command.Val = int64(val)
		case "R", "r":
			command.CmdType = cmdType
			command.Val = int64(val)
		default:
			err = InvalidShapeSvgStringError(s.ShapeSvgString)
			return
		}

		commands = append(commands, command)

		normSvg = strings.Replace(normSvg, cmdString, "", 1)
		normSvg = strings.Trim(normSvg, " ")
		if normSvg == "" {
			break
		}
	}

	return
}

func (s Shape) getPathCommands() (commands []PathCommand, err error) {
	normSvg := normalizeSvgString(s.ShapeSvgString)
	for {
		command := PathCommand{}

		re := regexp.MustCompile("(^.+?)([a-zA-Z])(.*)")
		cmdString := strings.Trim(re.ReplaceAllString(normSvg, "$1"), " ")

		pos := strings.Split(string(cmdString[1:]), ",")
		posEmpty := len(pos) <= 1 && pos[0] == ""

		cmdType := string(cmdString[0])
		switch cmdType {
		case "M", "m":
			command.CmdType = cmdType

			if len(pos) < 2 || posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else if s.Fill != "transparent" {
				if pathCommandExists(PathCommand{CmdType: "M"}, commands) || pathCommandExists(PathCommand{CmdType: "m"}, commands) {
					err = InvalidShapeSvgStringError(s.ShapeSvgString)
					return
				}
			}

			command.X, _ = strconv.ParseInt(pos[0], 10, 64)
			command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
		case "H":
			command.CmdType = "H"

			if posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.X, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "V":
			command.CmdType = "V"

			if posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.Y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "L":
			command.CmdType = "L"

			if len(pos) < 2 || posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.X, _ = strconv.ParseInt(pos[0], 10, 64)
				command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "h":
			command.CmdType = "h"

			if posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.X, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "v":
			command.CmdType = "v"

			if posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.Y, _ = strconv.ParseInt(pos[0], 10, 64)
			}
		case "l":
			command.CmdType = "l"

			if len(pos) < 2 || posEmpty {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
				return
			} else {
				command.X, _ = strconv.ParseInt(pos[0], 10, 64)
				command.Y, _ = strconv.ParseInt(pos[1], 10, 64)
			}
		case "Z", "z":
			command.CmdType = cmdType
		default:
			err = InvalidShapeSvgStringError(s.ShapeSvgString)
			return
		}

		commands = append(commands, command)

		normSvg = strings.Replace(normSvg, cmdString, "", 1)
		normSvg = strings.Trim(normSvg, " ")
		if normSvg == "" {
			break
		}
	}

	return
}

//Gets the shape geometry of a a provided shape
func (s Shape) GetGeometry() (geometry ShapeGeometry, err error) {
	if s.isCircle() {
		geometry, err = s.getCircleGeometry()
	} else if s.isPath() {
		geometry, err = s.getPathGeometry()
	}

	return
}

func (s Shape) getCircleGeometry() (geometry CircleGeometry, err error) {
	commands, err := s.getCircleCommands()
	if err != nil {
		return
	}

	geometry = CircleGeometry{
		ShapeSvgString: s.ShapeSvgString,
		Fill:           s.Fill,
		Min:            Point{},
		Max:            Point{}}

	for i := range commands {
		command := commands[i]

		switch command.CmdType {
		case "X", "x":
			geometry.Center.X = int64(command.Val)
		case "Y", "y":
			geometry.Center.Y = int64(command.Val)
		case "R", "r":
			geometry.Radius = command.Val
		default:
			err = InvalidShapeSvgStringError(s.ShapeSvgString)
			return
		}
	}

	geometry.Min.X, geometry.Min.Y = geometry.Center.X-geometry.Radius, geometry.Center.Y-geometry.Radius
	geometry.Max.X, geometry.Max.Y = geometry.Center.X+geometry.Radius, geometry.Center.Y+geometry.Radius

	return
}

func (s Shape) getPathGeometry() (geometry PathGeometry, err error) {
	commands, err := s.getPathCommands()
	if err != nil {
		return
	}

	geometry = PathGeometry{
		ShapeSvgString: s.ShapeSvgString,
		Fill:           s.Fill,
		Min:            Point{},
		Max:            Point{}}

	absPos, relPos := Point{0, 0}, Point{0, 0}
	var currentVertices []Point
	for i := range commands {
		command := commands[i]

		switch command.CmdType {
		case "M":
			absPos.X, absPos.Y = command.X, command.Y
			relPos.X, relPos.Y = command.X, command.Y

			if len(currentVertices) > 0 {
				geometry.VertexSets = append(geometry.VertexSets, currentVertices)
				currentVertices = []Point{}
			}

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "m":
			absPos.X, absPos.Y = relPos.X+command.X, relPos.Y+command.Y
			relPos.X, relPos.Y = absPos.X, absPos.Y

			if len(currentVertices) > 0 {
				geometry.VertexSets = append(geometry.VertexSets, currentVertices)
				currentVertices = []Point{}
			}

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "H":
			relPos.X = command.X

			currentVertices = append(currentVertices, Point{relPos.X, absPos.Y})
		case "V":
			relPos.Y = command.Y

			currentVertices = append(currentVertices, Point{absPos.X, relPos.Y})
		case "L":
			relPos.X, relPos.Y = command.X, command.Y

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "h":
			relPos.X = relPos.X + command.X

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "v":
			relPos.Y = relPos.Y + command.Y

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "l":
			relPos.X, relPos.Y = relPos.X+command.X, relPos.Y+command.Y

			currentVertices = append(currentVertices, Point{relPos.X, relPos.Y})
		case "Z":
			currentVertices = append(currentVertices, currentVertices[0])

			geometry.VertexSets = append(geometry.VertexSets, currentVertices)
			currentVertices = []Point{}
		case "z":
			currentVertices = append(currentVertices, currentVertices[0])

			geometry.VertexSets = append(geometry.VertexSets, currentVertices)
			currentVertices = []Point{}
		default:
			err = InvalidShapeSvgStringError(s.ShapeSvgString)
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

	if len(currentVertices) > 0 {
		geometry.VertexSets = append(geometry.VertexSets, currentVertices)
	}

	// Make sure its closed
	if s.Fill != "transparent" {
		if len(geometry.VertexSets) > 1 {
			err = InvalidShapeSvgStringError(s.ShapeSvgString)
		} else {
			firstVertex := geometry.VertexSets[0][0]
			lastVertex := geometry.VertexSets[0][len(geometry.VertexSets[0])-1]

			if firstVertex != lastVertex {
				err = InvalidShapeSvgStringError(s.ShapeSvgString)
			}
		}

		if err != nil {
			return
		}
	}

	geometry.LineSegmentSets = make([]LineSegmentSet, len(geometry.VertexSets))
	for i, vSet := range geometry.VertexSets {
		geometry.LineSegmentSets[i] = getLineSegments(vSet)
	}

	return
}

// </SHAPE>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <SHAPE GEOMETRY>

type ShapeGeometry interface {
	GetInkCost() (inkUnits uint64)
	isValid(xMax uint32, yMax uint32) (valid bool, err error)
	HasOverlap(_s ShapeGeometry) bool
	containsVertex(vertices []Point) bool
}

////////////////////////////////////////////////////////////////////////////////////////////
//			<PATH GEOMETRY>

type PathGeometry struct {
	ShapeSvgString string
	Fill           string
	Stroke         string

	VertexSets      []VertexSet
	LineSegmentSets []LineSegmentSet
	Min             Point
	Max             Point
}

type VertexSet []Point
type LineSegmentSet []LineSegment

func (s PathGeometry) getAllLineSegments() (lineSegments []LineSegment) {
	for _, _lineSegments := range s.LineSegmentSets {
		for _, lineSegment := range _lineSegments {
			lineSegments = append(lineSegments, lineSegment)
		}
	}

	return
}

func (s PathGeometry) getAllVertices() (vertices []Point) {
	for _, _vertices := range s.VertexSets {
		for _, vertex := range _vertices {
			vertices = append(vertices, vertex)
		}
	}

	return
}

func (s PathGeometry) computePerimeter() (perimiter uint64) {
	lineSegments := s.getAllLineSegments()
	for _, lineSegment := range lineSegments {
		perimiter = perimiter + lineSegment.Length()
	}

	return
}

// Computes the total area within a polygon using a scanline
// descending down the y-axis
// NOTE: This computes the actual number of pixels required to draw shape
// Doesn't exlude the actual line segments
func (s PathGeometry) computeArea() (area uint64) {
	lineSegments := s.LineSegmentSets[0]
	for y := s.Min.Y; y <= s.Max.Y; y++ {
		var intersects []Point

		scanLine := getLineSegment(Point{s.Min.X, y}, Point{s.Max.X, y})

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

// Computes the ink required for the given shape according
// to the fill specification.
func (s PathGeometry) GetInkCost() (inkUnits uint64) {
	if s.Fill == "transparent" {
		inkUnits = s.computePerimeter()
	} else {
		inkUnits = s.computeArea()
	}

	return
}

// Determines if the following conditions hold:
// - The shape is within the given bounding requirements
// - The shape is non-overlapping if not transparent
func (s PathGeometry) isValid(xMax uint32, yMax uint32) (valid bool, err error) {
	valid = true

	for _, vertex := range s.getAllVertices() {
		if valid = vertex.inBound(xMax, yMax); !valid {
			err = new(OutOfBoundsError)
			return
		}
	}

	if s.Fill != "transparent" {
		lineSegments := s.LineSegmentSets[0]
		for i := range lineSegments {
			curSeg := lineSegments[i]

			for j := range lineSegments {
				if i != j && curSeg.Intersects(lineSegments[j]) == true {
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
func (g PathGeometry) HasOverlap(_g ShapeGeometry) bool {
	if strings.HasSuffix(reflect.TypeOf(_g).String(), "PathGeometry") {
		_gP, _ := _g.(PathGeometry)
		return g.hasPathOverlap(_gP)
	} else {
		_gC, _ := _g.(CircleGeometry)
		return g.hasCircleOverlap(_gC)
	}

	return false
}

func (g PathGeometry) hasPathOverlap(_g PathGeometry) (overlap bool) {
	if intersectExists(g.getAllLineSegments(), _g.getAllLineSegments()) {
		overlap = true
	} else if g.Fill != "transparent" && g.containsVertex(_g.getAllVertices()) {
		overlap = true
	} else if _g.Fill != "transparent" && _g.containsVertex(g.getAllVertices()) {
		overlap = true
	}

	return
}

func (g PathGeometry) hasCircleOverlap(_g CircleGeometry) bool {
	return _g.HasOverlap(g)
}

// Determines if any of the vertices are contained with a polygon, using a scanline.
func (s PathGeometry) containsVertex(vertices []Point) bool {
	min := s.Min
	max := s.Max
	lineSegments := s.getAllLineSegments()

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

//			</PATH GEOMETRY>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
//			<CIRCLE GEOMETRY>
type CircleGeometry struct {
	ShapeSvgString string
	Fill           string
	Stroke         string

	Radius int64
	Center Point
	Min    Point
	Max    Point
}

func (g CircleGeometry) getLineIntersects(l LineSegment) (intersects []Point) {
	xC, yC, r := float64(g.Center.X), float64(g.Center.Y), float64(g.Radius)
	if l.A == 0 { // y is constant
		var y float64 = float64(l.C / l.B)

		intersects = solveCircleForX(y, xC, yC, r, l)
	} else if l.B == 0 { // x is constant
		var x float64 = float64(l.C / l.A)

		intersects = solveCircleForY(x, xC, yC, r, l)
	} else {
		lA_lB := float64(l.A / l.B)
		lC_lB := float64(l.C / l.B)

		var a float64 = float64(lA_lB*lA_lB) + 1.0
		var b float64 = (-2.0 * (xC)) - (2.0 * (lA_lB * (lC_lB * yC)))
		var c float64 = (xC * xC) + math.Pow(lC_lB-yC, 2) - (r * r)

		var d float64 = (b * b) - (4.0 * a * c)
		if d < 0 {
			return
		} else if d == 0 {
			var x float64 = (-1 * b) / (2 * a)

			intersects = solveCircleForY(x, xC, yC, r, l)
		} else {
			_d := math.Sqrt(float64(d))

			var x1 float64 = (float64(-1*b) + _d) / float64(2*a)
			var x2 float64 = (float64(-1*b) - _d) / float64(2*a)

			p1 := solveCircleForY(x1, xC, yC, r, l)
			p2 := solveCircleForY(x2, xC, yC, r, l)

			if len(p1) > 0 {
				intersects = append(intersects, p1[0])
			}

			if len(p2) > 0 {
				intersects = append(intersects, p2[0])
			}
		}
	}

	return
}

func (g CircleGeometry) computePerimeter() (perimeter uint64) {
	return uint64(math.Ceil(2 * math.Pi * float64(g.Radius)))
}

func (g CircleGeometry) computeArea() (area uint64) {
	for y := g.Min.Y; y <= g.Max.Y; y++ {
		scanLine := getLineSegment(Point{g.Min.X, y}, Point{g.Max.X, y})
		intersects := g.getLineIntersects(scanLine)
		if len(intersects) > 1 {
			lineSegment := getLineSegment(intersects[0], intersects[1])

			area = area + lineSegment.Length()
		} else if len(intersects) == 1 {
			area = area + 1
		}
	}

	return
}

func (g CircleGeometry) GetInkCost() (inkUnits uint64) {
	if g.Fill == "transparent" {
		inkUnits = g.computePerimeter()
	} else {
		inkUnits = g.computeArea()
	}

	return
}
func (g CircleGeometry) isValid(xMax uint32, yMax uint32) (valid bool, err error) {
	if g.Min.inBound(xMax, yMax) && g.Max.inBound(xMax, yMax) {
		return true, nil
	} else {
		return false, new(OutOfBoundsError)
	}
}

func (g CircleGeometry) HasOverlap(_g ShapeGeometry) bool {
	if strings.HasSuffix(reflect.TypeOf(_g).String(), "PathGeometry") {
		_gP, _ := _g.(PathGeometry)
		return g.hasPathOverlap(_gP)
	} else {
		_gC, _ := _g.(CircleGeometry)
		return g.hasCircleOverlap(_gC)
	}

	return false
}

func (g CircleGeometry) hasPathOverlap(_g PathGeometry) bool {
	vertices := _g.getAllVertices()
	lineSegments := _g.getAllLineSegments()

	// Does the circle contain any of the polygons vertices?
	if g.Fill != "transparent" && g.containsVertex(vertices) {
		return true
	}

	// Does the circle intersect any of the polygons line segments?
	for _, l := range lineSegments {
		if len(g.getLineIntersects(l)) > 0 {
			return true
		}
	}

	// Does the polygon contain the circle?
	if _g.Fill != "transparent" && _g.containsVertex([]Point{g.Center}) {
		return true
	}

	return false
}

func (g CircleGeometry) hasCircleOverlap(_g CircleGeometry) bool {
	// If they are same size, check distance between centers against radii.
	if g.Radius == _g.Radius {
		if dist := g.Center.getDist(_g.Center); dist <= float64(g.Radius+_g.Radius) {
			return true
		}
	} else {
		var smaller, bigger CircleGeometry
		if _g.Radius < g.Radius {
			smaller, bigger = _g, g
		} else {
			smaller, bigger = g, _g
		}

		dist := smaller.Center.getDist(bigger.Center)
		if dist > float64(smaller.Radius+bigger.Radius) {
			return false
		} else if bigger.Fill != "transparent" {
			return true
		}

		_dist := dist - float64(bigger.Radius)
		if _dist > 0 && _dist <= float64(smaller.Radius) {
			return true
		}
	}

	return false
}

func (g CircleGeometry) containsVertex(vertices []Point) bool {
	for _, v := range vertices {
		if g.Center.getDist(v) <= float64(g.Radius) {
			return true
		}
	}

	return false
}

//			</CIRCLE GEOMETRY>
////////////////////////////////////////////////////////////////////////////////////////////

// </SHAPE GEOMETRY>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <POINT>

// Represents a point with (x, y) coordinate
type Point struct {
	X int64
	Y int64
}

func (p Point) inBound(xMax uint32, yMax uint32) bool {
	return p.X > 0 && p.Y > 0 && p.X < int64(xMax) && p.Y < int64(yMax)
}

func (p Point) getDist(_p Point) float64 {
	x1, x2, y1, y2 := p.X, _p.X, p.Y, _p.Y
	return math.Sqrt(math.Pow(float64(x2-x1), 2) + math.Pow(float64(y2-y1), 2))
}

// </POINT>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <LINE SEGMENT>

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

// Determines if a point lies on a line segment -- Assumes point found by intersection
func (l LineSegment) HasPoint(p Point) bool {
	x1, y1, x2, y2 := l.Start.X, l.Start.Y, l.End.X, l.End.Y

	return ((y1 <= p.Y && p.Y <= y2) || (y1 >= p.Y && p.Y >= y2)) &&
		((x1 <= p.X && p.X <= x2) || (x1 >= p.X && p.X >= x2))
}

// Determines if a point lies on a line segment -- Assumes point found by intersection
func (l LineSegment) hasFloatPoint(x float64, y float64) bool {
	x1, y1, x2, y2 := float64(l.Start.X), float64(l.Start.Y), float64(l.End.X), float64(l.End.Y)

	return ((y1 <= y && y <= y2) || (y1 >= y && y >= y2)) &&
		((x1 <= x && x <= x2) || (x1 >= x && x >= x2))
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

// </LINE SEGMENT>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <FUNCTIONS>

// Determines if the given command exists in a set of commands
func pathCommandExists(c PathCommand, commands []PathCommand) bool {
	for _, command := range commands {
		if c.CmdType == command.CmdType {
			return true
		}
	}

	return false
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
	for i := 0; i < len(vertices)-1; i++ {
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

func solveCircleForY(x float64, xC float64, yC float64, r float64, onLineSegment LineSegment) (points []Point) {
	var _points [][]float64

	var a float64 = 1.0
	var b float64 = -2.0 * yC
	var c float64 = (yC * yC) + math.Pow(x-xC, 2) - (r * r)

	d := float64((b * b) - (4.0 * a * c))
	if d < 0 {
		return
	} else if d == 0 {
		var y float64 = (-1 * b) / (2 * a)

		_points = append(_points, []float64{x, y})
	} else {
		_d := math.Sqrt(float64(d))

		var y1 float64 = ((-1 * b) + _d) / (2 * a)
		var y2 float64 = ((-1 * b) - _d) / (2 * a)

		_points = append(_points, []float64{x, y1}, []float64{x, y2})
	}

	for _, p := range _points {
		if onLineSegment.hasFloatPoint(p[0], p[1]) {
			x := int64(math.Ceil(p[0]))
			y := int64(math.Ceil(p[1]))

			points = append(points, Point{x, y})
		}
	}

	return
}

func solveCircleForX(y float64, xC float64, yC float64, r float64, onLineSegment LineSegment) (points []Point) {
	var _points [][]float64

	var a float64 = 1.0
	var b float64 = -2.0 * xC
	var c float64 = (xC * xC) + math.Pow(y-yC, 2) - (r * r)

	d := float64((b * b) - (4.0 * a * c))
	if d < 0 {
		return
	} else if d == 0 {
		var x float64 = (-1 * b) / (2 * a)

		_points = append(_points, []float64{x, y})
	} else {
		_d := math.Sqrt(float64(d))

		var x1 float64 = ((-1 * b) + _d) / (2 * a)
		var x2 float64 = ((-1 * b) - _d) / (2 * a)

		_points = append(_points, []float64{x1, y}, []float64{x2, y})
	}

	for _, p := range _points {
		if onLineSegment.hasFloatPoint(p[0], p[1]) {
			x := int64(math.Ceil(p[0]))
			y := int64(math.Ceil(p[1]))

			points = append(points, Point{x, y})
		}
	}

	return
}

// </FUNCTIONS>
////////////////////////////////////////////////////////////////////////////////////////////
