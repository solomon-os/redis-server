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

type RangeArgs struct {
	Key   string
	Start int // index to start listing
	End   int // index to stop listing
}

type XRangeArgs struct {
	Key   string
	Start string
	End   string
}

type XReadArgs struct {
	Command string
	Timeout int
	Streams []streamArgs
}

type streamArgs struct {
	Key   string
	Start string
}

type LenArgs struct {
	Key string
}

type PopArgs struct {
	Key       string
	Length    int
	Arguments bool
	Block     bool
	Timeout   float64
}

type TypeArgs struct {
	Key string
}

type StreamArgs struct {
	Key    string
	ID     string
	Fields map[string]string
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

func ParseRangeArgs(cmd Command) RangeArgs {
	return RangeArgs{
		Key:   cmd.Args[0],
		Start: parseInt(cmd.Args[1]),
		End:   parseInt(cmd.Args[2]),
	}
}

func ParseXRangeArgs(cmd Command) (XRangeArgs, error) {
	args := XRangeArgs{}
	if len(cmd.Args) < 2 {
		return args, errors.New("invalid command arguments")
	}

	args.Key = cmd.Args[0]
	args.Start = cmd.Args[1]
	args.End = ""

	if len(cmd.Args) > 2 {
		args.End = cmd.Args[2]
	}
	return args, nil
}

func ParseXReadArgs(cmd Command) (XReadArgs, error) {
	args := XReadArgs{}

	if len(cmd.Args) < 3 {
		return args, errors.New("wrong command arguments for xread")
	}

	switch strings.ToUpper(cmd.Args[0]) {
	case "STREAMS":
		args.Command = "STREAMS"
		arr := cmd.Args[1:]
		n := len(arr)

		if n%2 != 0 {
			return args, errors.New("wrong command arguments for xread")
		}

		args.Streams = make([]streamArgs, 0, n/2)

		for i, j := 0, n/2; j < n; i, j = i+1, j+1 {
			args.Streams = append(args.Streams, streamArgs{Key: arr[i], Start: arr[j]})
		}

		return args, nil

	case "BLOCK":
		args.Command = "BLOCK"
		timeout, err := strconv.Atoi(cmd.Args[1])
		if err != nil {
			return args, errors.New("invalid timeout argument")
		}
		args.Timeout = timeout

		recursedArg, err := ParseXReadArgs(Command{Name: cmd.Name, Args: cmd.Args[2:]})
		if err != nil {
			return args, err
		}
		args.Streams = recursedArg.Streams
	}

	return args, errors.New("invalid xread command")
}

func ParseLenArgs(cmd Command) LenArgs {
	return LenArgs{
		Key: cmd.Args[0],
	}
}

func ParsePopArgs(cmd Command) PopArgs {
	args := PopArgs{
		Key:       cmd.Args[0],
		Length:    0,
		Arguments: false,
	}

	if len(cmd.Args) > 1 {
		args.Length = parseInt(cmd.Args[1])
		args.Arguments = true
	}

	return args
}

func ParseBPopArgs(cmd Command) PopArgs {
	return PopArgs{
		Key:     cmd.Args[0],
		Timeout: parseFloat(cmd.Args[1]),
	}
}

func ParseTypeArgs(cmd Command) TypeArgs {
	return TypeArgs{
		Key: cmd.Args[0],
	}
}

func ParseStreamArgs(cmd Command) (StreamArgs, error) {
	args := StreamArgs{
		Key: cmd.Args[0],
		ID:  cmd.Args[1],
	}

	fields := make(map[string]string)

	entries := cmd.Args[2:]

	if len(entries)%2 != 0 {
		return args, errors.New("wrong number of arguments")
	}

	for i := 0; i < len(entries); i += 2 {
		fields[entries[i]] = entries[i+1]
	}

	args.Fields = fields

	return args, nil
}

func parseInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		slog.Error("convertion to int failed", "value", s, "error", err)
		return -1
	}

	return val
}

func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		slog.Error("convertion to int failed", "value", s, "error", err)
		return -1
	}

	return val
}
