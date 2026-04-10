package resp

import "fmt"

func SimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}

func BulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
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
