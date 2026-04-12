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

type SetArgs struct {
	Key   string
	Value string
	TTL   int64 // stores expire in milliseconds
}

type PushArgs struct {
	Key   string
	Value []string
}

type LRangeArgs struct {
	Key   string
	Start int // index to start listing
	End   int // index to stop listing
}

type LenArgs struct {
	Key string
}

func Parse(input string) (Command, error) {
	firstChar := input[0]

	switch firstChar {
	case '*':
		return parseArray(input)
	}
	return Command{}, errors.New("unknown message type")
}

// Clients send commands to the Redis server as RESP arrays. Similarly,
// some Redis commands that return collections of elements use arrays as their replies.
// An example is the LRANGE command that returns elements of a list.
// RESP Arrays' encoding uses the following format:
// *<number-of-elements>\r\n<element-1>...<element-n>
func parseArray(input string) (Command, error) {
	slog.Info(fmt.Sprintf("%q", input))
	// I know, I know there's a more efficient way for parsing the string instead of splitting
	// and allocating an array for each segment of the string
	// but this is not an hft project my guy, it's a toy project and it's not that serious.
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

func ParseSetArgs(cmd Command) SetArgs {
	setArgs := SetArgs{
		Key:   cmd.Args[0],
		Value: cmd.Args[1],
		TTL:   -1,
	}

	if len(cmd.Args) < 4 {
		return setArgs
	}

	// The PX option is used to set a key's expiry time in milliseconds. After the key expires, it's no longer accessible.
	switch strings.ToLower(cmd.Args[2]) {
	case "px":
		ttl, err := strconv.ParseInt(cmd.Args[3], 10, 64)
		if err != nil {
			slog.Error("convertion px value int failed: ", "error", err)
			return setArgs
		}

		if ttl > 0 {
			setArgs.TTL = ttl
		}

		return setArgs

	// The EX option is used to set a key's expiry time in seconds. After the key expires, it's no longer accessible.
	case "ex":
		ttl, err := strconv.ParseInt(cmd.Args[3], 10, 64)
		if err != nil {
			slog.Error("convertion px value int failed: ", "error", err)
			return setArgs
		}

		if ttl > 0 {
			setArgs.TTL = ttl * 1000
		}

		return setArgs
	}
	return setArgs
}

func ParsePushArgs(cmd Command) PushArgs {
	return PushArgs{
		Key:   cmd.Args[0],
		Value: cmd.Args[1:],
	}
}

func ParseLRangeArgs(cmd Command) LRangeArgs {
	return LRangeArgs{
		Key:   cmd.Args[0],
		Start: parseInt(cmd.Args[1]),
		End:   parseInt(cmd.Args[2]),
	}
}

func ParseLenArgs(cmd Command) LenArgs {
	return LenArgs{
		Key: cmd.Args[0],
	}
}

func parseInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		slog.Error("convertion to int failed", "value", s, "error", err)
		return -1
	}

	return val
}
