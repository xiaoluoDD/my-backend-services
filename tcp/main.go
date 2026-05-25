package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	onlineMu sync.Mutex
	online    = 0
)

func main() {
	const addr = ":8889"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	log.Printf("server (with commands) listening on %s\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handleCmdConn(conn)
	}
}

func handleCmdConn(conn net.Conn) {
	defer conn.Close()
	onlineMu.Lock()
	online++
	cur := online
	onlineMu.Unlock()

	log.Printf("client connected (online=%d) from %s\n", cur, conn.RemoteAddr())

	rd := bufio.NewReader(conn)
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			break
		}
		cmd := strings.TrimSpace(line)

		var reply string
		switch {
		case cmd == "time":
			reply = time.Now().Format("2006-01-02 15:04:05") + "\n"
		case cmd == "online":
			onlineMu.Lock()
			n := online
			onlineMu.Unlock()
			reply = fmt.Sprintf("online=%d\n", n)
		case strings.HasPrefix(cmd, "echo "):
			reply = strings.TrimPrefix(cmd, "echo ") + "\n"
		default:
			reply = "received: " + cmd + "\n"
		}

		if _, werr := conn.Write([]byte(reply)); werr != nil {
			break
		}
	}

	onlineMu.Lock()
	online--
	log.Printf("client disconnected (online=%d)\n", online)
}
