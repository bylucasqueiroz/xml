package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type ttype byte

const (
	typObject ttype = iota
	typArray
	typBool
	typNumber
	typString
	typNull
)

var ttypeNames = [...]string{"object", "array", "boolean", "number", "string", "null"}

type JSONDecoder interface {
	Token() (json.Token, error)
}

type XMLEncoder interface {
	EncodeToken(xml.Token) error
}

type Converter struct {
	decoder   JSONDecoder
	types     []ttype
	data      *string
	keyNames  []string
	arrayKeys []string
}

func Tokens(j JSONDecoder) *Converter {
	return &Converter{
		decoder: j,
	}
}

func (c *Converter) Token() (xml.Token, error) {
	if c.data != nil {
		token := xml.CharData(*c.data)
		c.data = nil
		return token, nil
	}
	if len(c.types) > 0 {
		switch c.types[len(c.types)-1] {
		case typObject, typArray:
		default:
			return c.outputEnd(), nil
		}
	}
	token, err := c.decoder.Token()
	if err != nil {
		return nil, err
	}
	if len(c.types) > 0 && c.types[len(c.types)-1] == typObject && token != json.Delim('}') {
		tokenStr, ok := token.(string)
		if !ok {
			return nil, ErrInvalidKey
		}
		c.keyNames = append(c.keyNames, tokenStr)
		token, err = c.decoder.Token()
		if err != nil {
			return nil, err
		}
	}
	switch token := token.(type) {
	case json.Delim:
		switch token {
		case '{':
			return c.outputStart(typObject), nil
		case '[':
			if len(c.keyNames) > 0 {
				c.arrayKeys = append(c.arrayKeys, c.keyNames[len(c.keyNames)-1])
			}
			return c.outputStart(typArray), nil
		case '}':
			if len(c.types) == 0 || c.types[len(c.types)-1] != typObject {
				return nil, ErrInvalidToken
			}
			return c.outputEnd(), nil
		case ']':
			if len(c.types) == 0 || c.types[len(c.types)-1] != typArray {
				return nil, ErrInvalidToken
			}
			if len(c.arrayKeys) > 0 {
				c.arrayKeys = c.arrayKeys[:len(c.arrayKeys)-1]
			}
			return c.outputEnd(), nil
		default:
			return nil, ErrUnknownToken
		}
	case bool:
		return c.outputType(typBool, strconv.FormatBool(token)), nil
	case float64:
		return c.outputType(typNumber, strconv.FormatFloat(token, 'f', -1, 64)), nil
	case json.Number:
		return c.outputType(typNumber, string(token)), nil
	case string:
		return c.outputType(typString, token), nil
	case nil:
		return c.outputType(typNull, ""), nil
	default:
		return nil, ErrUnknownToken
	}
}

func (c *Converter) outputType(typ ttype, data string) xml.Token {
	c.data = &data
	return c.outputStart(typ)
}

func (c *Converter) outputStart(typ ttype) xml.Token {
	c.types = append(c.types, typ)
	if len(c.keyNames) > 0 {
		lastKey := c.keyNames[len(c.keyNames)-1]
		if len(c.arrayKeys) > 0 && lastKey == c.arrayKeys[len(c.arrayKeys)-1] {
			return xml.StartElement{
				Name: xml.Name{
					Local: lastKey,
				},
			}
		}
		c.keyNames = c.keyNames[:len(c.keyNames)-1] // Remove key from keyNames when starting an element
		return xml.StartElement{
			Name: xml.Name{
				Local: lastKey,
			},
		}
	}
	return xml.StartElement{
		Name: xml.Name{
			Local: ttypeNames[typ],
		},
	}
}

func (c *Converter) outputEnd() xml.Token {
	typ := c.types[len(c.types)-1]
	c.types = c.types[:len(c.types)-1]
	if len(c.keyNames) > 0 && len(c.arrayKeys) > 0 && c.keyNames[len(c.keyNames)-1] == c.arrayKeys[len(c.arrayKeys)-1] {
		return xml.EndElement{
			Name: xml.Name{
				Local: c.arrayKeys[len(c.arrayKeys)-1],
			},
		}
	}
	if len(c.keyNames) > 0 {
		lastKey := c.keyNames[len(c.keyNames)-1]
		c.keyNames = c.keyNames[:len(c.keyNames)-1]
		return xml.EndElement{
			Name: xml.Name{
				Local: lastKey,
			},
		}
	}
	return xml.EndElement{
		Name: xml.Name{
			Local: ttypeNames[typ],
		},
	}
}

func Convert(j JSONDecoder, x XMLEncoder) error {
	c := Converter{
		decoder: j,
	}

	// Add root element start
	startElement := xml.StartElement{Name: xml.Name{Local: "root"}}
	if err := x.EncodeToken(startElement); err != nil {
		return err
	}

	for {
		tk, err := c.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if tk != nil {
			if err = x.EncodeToken(tk); err != nil {
				return err
			}
		}
	}

	// Add root element end
	endElement := xml.EndElement{Name: xml.Name{Local: "root"}}
	if err := x.EncodeToken(endElement); err != nil {
		return err
	}

	return nil
}

var (
	cTrue  = "true"
	cFalse = "false"
)

var (
	ErrInvalidKey   = errors.New("invalid key type")
	ErrUnknownToken = errors.New("unknown token type")
	ErrInvalidToken = errors.New("invalid token")
)

func main() {
	decoder := json.NewDecoder(strings.NewReader(jsonData))
	encoder := xml.NewEncoder(os.Stdout)
	encoder.Indent("", "  ")

	if err := Convert(decoder, encoder); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	encoder.Flush()
}
