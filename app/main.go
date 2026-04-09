package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/codecrafters-io/redis-starter-go/app/internal/parser"
)

// Ensures gofmt doesn't remove the "net" and "os" imports in stage 1 (feel free to remove this!)
var (
	_ = net.Listen
	_ = os.Exit
)

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	// Uncomment the code below to pass the first stage
	//

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
	p := parser.New()

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("%v", err)
			return
		}

		p, err = p.Parse(string(buf[:n]), n)
		if err != nil {
			slog.Error("couldn't parse message: ", "error", err)
			return
		}
		resp := p.Decode()
		fmt.Printf("%#v", p)
		fmt.Printf("%q\n", []byte(resp))

		_, err = io.WriteString(conn, resp)
		if err != nil {
			slog.Error("couldn't send response: ", "error", err)
			return
		}
	}
}
