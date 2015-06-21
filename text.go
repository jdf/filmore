package filmore

import (
	"log"

	"io/ioutil"

	"code.google.com/p/freetype-go/freetype/truetype"
)

// Scaling constant for going from points to pixels.
const DPI = 92

type Op interface {
	X() float64
	Y() float64
	ControlX() float64
	ControlY() float64
}

type op struct{ x, y float64 }

type MoveTo op
type LineTo op
type QuadCurveTo struct {
	x, y, cx, cy float64
}

func (o MoveTo) X() float64        { return o.x }
func (o MoveTo) Y() float64        { return o.y }
func (o MoveTo) ControlX() float64 { return o.x }
func (o MoveTo) ControlY() float64 { return o.y }

func (o LineTo) X() float64        { return o.x }
func (o LineTo) Y() float64        { return o.y }
func (o LineTo) ControlX() float64 { return o.x }
func (o LineTo) ControlY() float64 { return o.y }

func (o QuadCurveTo) X() float64        { return o.x }
func (o QuadCurveTo) Y() float64        { return o.y }
func (o QuadCurveTo) ControlX() float64 { return o.cx }
func (o QuadCurveTo) ControlY() float64 { return o.cy }

type Font struct {
	font     *truetype.Font
	glyphBuf *truetype.GlyphBuf
	scale    int32
}

type TextPath struct {
	PathOps []Op
	Width   float64
}

func (p *TextPath) MoveTo(x, y float64) {
	p.PathOps = append(p.PathOps, MoveTo{x, y})
}

func (p *TextPath) LineTo(x, y float64) {
	p.PathOps = append(p.PathOps, LineTo{x, y})
}

func (p *TextPath) QuadCurveTo(x, y, controlX, controlY float64) {
	p.PathOps = append(p.PathOps, QuadCurveTo{x, y, controlX, controlY})
}

func ttscale(fontSize int) int32 {
	return int32(float64(fontSize) * float64(DPI) * (64.0 / 72.0))
}

func NewFont(fontData []byte, fontSize int) (*Font, error) {
	font, err := truetype.Parse(fontData)
	if err != nil {
		return nil, err
	}
	return &Font{font, truetype.NewGlyphBuf(), ttscale(fontSize)}, nil
}

func NewFontFromFile(filename string, fontSize int) (*Font, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return NewFont(data, fontSize)
}

func fUnitsToFloat64(x int32) float64 {
	scaled := x << 2
	return float64(scaled/256) + float64(scaled%256)/256.0
}

// p is a truetype.Point measured in FUnits and positive Y going upwards.
// The returned value is the same thing measured in floating point and positive Y
// going downwards.
func pointToF64Point(p truetype.Point) (x, y float64) {
	return fUnitsToFloat64(p.X), -fUnitsToFloat64(p.Y)
}

func (textPath *TextPath) appendContour(ps []truetype.Point, dx, dy float64) {
	if len(ps) == 0 {
		return
	}
	startX, startY := pointToF64Point(ps[0])
	textPath.MoveTo(startX+dx, startY+dy)
	q0X, q0Y, on0 := startX, startY, true
	for _, p := range ps[1:] {
		qX, qY := pointToF64Point(p)
		on := p.Flags&0x01 != 0
		if on {
			if on0 {
				textPath.LineTo(qX+dx, qY+dy)
			} else {
				textPath.QuadCurveTo(q0X+dx, q0Y+dy, qX+dx, qY+dy)
			}
		} else {
			if on0 {
				// No-op.
			} else {
				midX := (q0X + qX) / 2
				midY := (q0Y + qY) / 2
				textPath.QuadCurveTo(q0X+dx, q0Y+dy, midX+dx, midY+dy)
			}
		}
		q0X, q0Y, on0 = qX, qY, on
	}
	// Close the curve.
	if on0 {
		textPath.LineTo(startX+dx, startY+dy)
	} else {
		textPath.QuadCurveTo(q0X+dx, q0Y+dy, startX+dx, startY+dy)
	}
}

func (f *Font) appendGlyphPath(glyph truetype.Index, dx, dy float64, textPath *TextPath) error {
	if err := f.glyphBuf.Load(f.font, f.scale, glyph, truetype.NoHinting); err != nil {
		return err
	}
	e0 := 0
	for _, e1 := range f.glyphBuf.End {
		textPath.appendContour(f.glyphBuf.Point[e0:e1], dx, dy)
		e0 = e1
	}
	return nil
}

// CreateTextPath creates a TextPath from the string s at x, y, and returns it.
// The text is placed so that the left edge of the em square of the first character of s
// and the baseline intersect at x, y. The majority of the affected pixels will be
// above and to the right of the point, but some may be below or to the left.
// For example, drawing a string that starts with a 'J' in an italic font may
// affect pixels below and left of the point.
func (f *Font) CreateTextPath(s string, x, y float64) TextPath {
	result := TextPath{}
	startx := x
	prev, hasPrev := truetype.Index(0), false
	for _, rune := range s {
		index := f.font.Index(rune)
		if hasPrev {
			x += fUnitsToFloat64(f.font.Kerning(f.scale, prev, index))
		}
		err := f.appendGlyphPath(index, x, y, &result)
		if err != nil {
			log.Println(err)
			return result
		}
		x += fUnitsToFloat64(f.font.HMetric(f.scale, index).AdvanceWidth)
		result.Width = x - startx
		prev, hasPrev = index, true
	}
	return result
}
