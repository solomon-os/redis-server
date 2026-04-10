package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/codecrafters-io/redis-starter-go/app/internal/parser"
	"github.com/codecrafters-io/redis-starter-go/app/internal/resp"
)

var (
	_ = net.Listen
	_ = os.Exit
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				slog.Error("read error", "error", err)
			}
			return
		}

		cmd, err := parser.Parse(string(buf[:n]))
		if err != nil {
			slog.Error("couldn't parse message", "error", err)
			io.WriteString(conn, resp.Error("invalid command"))
			return
		}

		response := handleCommand(cmd)
		_, err = io.WriteString(conn, response)
		if err != nil {
			slog.Error("couldn't send response", "error", err)
			return
		}
	}
}

func handleCommand(cmd parser.Command) string {
	switch cmd.Name {
	case "PING":
		return resp.SimpleString("PONG")
	case "ECHO":
		if len(cmd.Args) < 1 {
			return resp.Error("wrong number of arguments for 'echo' command")
		}
		return resp.BulkString(cmd.Args[0])
	case "SET":
		return resp.SimpleString("OK")
	default:
		return resp.Error("unknown command")
	}
}
