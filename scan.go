package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	ip := "162.159.192.1"
	port := "2408"
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), timeout)
	if err != nil {
		fmt.Println("Timeout or error:", err)
		return
	}
	conn.Close()
	fmt.Println("Connected OK!")
}
