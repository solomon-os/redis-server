package resp

import (
	"fmt"
	"strings"
)

type StreamReply struct {
	ID     string
	Fields []string
}

type ReadStreamsReply struct {
	Key           string
	StreamReplies []StreamReply
}

func SimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}

func BulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func NullOrBulkStringArray(s []string) string {
	if len(s) == 0 {
		return "*-1\r\n"
	}
	return BulkStringArray(s)
}

func BulkStringArray(s []string) string {
	var str strings.Builder

	fmt.Fprintf(&str, "*%d\r\n", len(s))

	for _, item := range s {
		str.WriteString(BulkString(item))
	}

	return str.String()
}

func XRangeReply(s []StreamReply) string {
	var str strings.Builder

	fmt.Fprintf(&str, "*%d\r\n", len(s))

	for i := range s {
		fmt.Fprintf(&str, "*%d\r\n", 2)
		str.WriteString(BulkString(s[i].ID))
		str.WriteString(BulkStringArray(s[i].Fields))
	}
	return str.String()
}

func XReadReply(s []ReadStreamsReply) string {
	var str strings.Builder

	if len(s) == 0 {
		return "*-1\r\n"
	}

	fmt.Fprintf(&str, "*%d\r\n", len(s))

	for i := range s {
		fmt.Fprintf(&str, "*%d\r\n", 2)
		str.WriteString(BulkString(s[i].Key))
		str.WriteString(XRangeReply(s[i].StreamReplies))
	}

	return str.String()
}

func NullBulkString() string {
	return "$-1\r\n"
}

func Integer(n int) string {
	return fmt.Sprintf(":%d\r\n", n)
}

func Error(msg string) string {
	return fmt.Sprintf("-ERR %s\r\n", msg)
}
