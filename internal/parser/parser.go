package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

type Command struct {
	Name string
	Args []string
}

func Parse(input string) (Command, error) {
	firstChar := input[0]

	switch firstChar {
	case '*':
		return parseArray(input)
	}
	return Command{}, errors.New("unknown message type")
}

func parseArray(input string) (Command, error) {
	slog.Info(fmt.Sprintf("%q", input))
	strArray := strings.Split(input, "\r\n")
	size, err := strconv.Atoi(strArray[0][1:])
	if err != nil {
		return Command{}, errors.New("invalid message sent")
	}

	args := make([]string, 0, size)
	for i := 2; i < len(strArray); i += 2 {
		args = append(args, strArray[i])
	}

	if len(args) == 0 {
		return Command{}, errors.New("empty command")
	}

	return Command{
		Name: strings.ToUpper(args[0]),
		Args: args[1:],
	}, nil
}
