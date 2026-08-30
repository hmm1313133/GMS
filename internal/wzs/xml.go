package wzs

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse reads one exported wz XML document (an "<imgdir name=...>" tree) and
// returns its root node. baseDir is only used to reconstruct canvas PNG paths,
// pass "" when parsing a bare stream.
func Parse(r io.Reader) (*Node, error) { return parseXML(r, "") }

func parseXML(r io.Reader, baseDir string) (*Node, error) {
	dec := xml.NewDecoder(bufio.NewReaderSize(r, 64<<10))
	var root *Node
	stack := make([]*Node, 0, 32)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n, err := newNode(t)
			if err != nil {
				return nil, err
			}
			if len(stack) == 0 {
				root = n
				n.baseDir = baseDir
			} else {
				stack[len(stack)-1].addChild(n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("wzs: unexpected closing tag </%s>", t.Name.Local)
			}
			done := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			done.finalize()
		}
	}
	if len(stack) != 0 {
		return nil, errors.New("wzs: unclosed tag at end of document")
	}
	if root == nil {
		return nil, errors.New("wzs: document has no element")
	}
	return root, nil
}

func attrOf(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func newNode(e xml.StartElement) (*Node, error) {
	n := &Node{Name: attrOf(e, "name"), Type: dataTypeOf(e.Name.Local)}
	switch n.Type {
	case TypeShort, TypeInt, TypeLong:
		raw := strings.TrimSpace(attrOf(e, "value"))
		if raw == "" { // missing / empty attribute: wz does carry a few of those
			break
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("wzs: <%s name=%q value=%q>: %w", e.Name.Local, n.Name, raw, err)
		}
		n.iv = v
	case TypeFloat, TypeDouble:
		raw := strings.TrimSpace(attrOf(e, "value"))
		if raw == "" {
			break
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("wzs: <%s name=%q value=%q>: %w", e.Name.Local, n.Name, raw, err)
		}
		n.fv = v
	case TypeString, TypeUOL:
		n.sv = attrOf(e, "value")
	case TypeVector:
		for i, axis := range []string{"x", "y"} {
			raw := strings.TrimSpace(attrOf(e, axis))
			if raw == "" {
				continue
			}
			v, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("wzs: <vector name=%q %s=%q>: %w", n.Name, axis, raw, err)
			}
			if i == 0 {
				n.pt.X = v
			} else {
				n.pt.Y = v
			}
		}
	case TypeCanvas:
		for i, dim := range []string{"width", "height"} {
			raw := strings.TrimSpace(attrOf(e, dim))
			if raw == "" {
				continue
			}
			v, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("wzs: <canvas name=%q %s=%q>: %w", n.Name, dim, raw, err)
			}
			if i == 0 {
				n.cv.Width = v
			} else {
				n.cv.Height = v
			}
		}
	}
	return n, nil
}
