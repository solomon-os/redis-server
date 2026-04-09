package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Parser struct {
	Command string
	Type    string
	Array   []string
}

const (
	ECHO_COMMAND = "ECHO"
	ARRAY_TYPE   = "Array"
)

func New() *Parser {
	p := Parser{}
	return &p
}

func (p *Parser) Parse(input string, size int) (*Parser, error) {
	// get the first character;
	firstChar := input[0]

	switch firstChar {
	case '*':
		err := p.parseArray(input, size)
		return p, err
	}
	return p, nil
}

// Clients send commands to the Redis server as RESP arrays. Similarly, some Redis commands that return collections of elements use arrays as their replies. An example is the LRANGE command that returns elements of a list.
// RESP Arrays' encoding uses the following format:
// *<number-of-elements>\r\n<element-1>...<element-n>
// example: *2\r\n$4\r\nECHO\r\n$3\r\nhey\r\n
func (p *Parser) parseArray(input string, _ int) error {
	strArray := strings.Split(input, "\r\n")
	// get the all the remaining characters of the first element excluding the first one
	size, err := strconv.Atoi(strArray[0][1:])
	if err != nil {
		return errors.New("invalid message sent")
	}

	p.Type = ARRAY_TYPE
	p.Array = make([]string, size)

	for i, j := 2, 0; i < len(strArray); i, j = i+2, j+1 {
		if i == 2 {
			p.Command = strings.ToUpper(strArray[i])
		}
		p.Array[j] = strArray[i]
	}

	return nil
}

func (p *Parser) Decode() string {
	if p.Command == ECHO_COMMAND {
		return fmt.Sprintf("$%d\r\n%s\r\n", len(p.Array[1]), p.Array[1])
	}

	return ""
}
