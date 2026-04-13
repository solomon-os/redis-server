package resp

import (
	"fmt"
	"strings"
)

func SimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}

func BulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func BulkStringArray(s []string) string {
	var str strings.Builder

	if len(s) == 0 {
		return "*-1\r\n"
	}

	fmt.Fprintf(&str, "*%d\r\n", len(s))

	for _, item := range s {
		str.WriteString(BulkString(item))
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
