package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"time"
)

const ipCount = 30
const port = "2408"
const timeout = 1200 * time.Millisecond

type Endpoint struct {
	IP      string
	Latency time.Duration
}

func randomIPv4() string {
	blocks := [][]int{
		{162, 159, 192},
		{162, 159, 193},
		{162, 159, 195},
		{188, 114, 96},
		{188, 114, 97},
		{188, 114, 98},
		{188, 114, 99},
	}
	blk := blocks[rand.Intn(len(blocks))]
	return fmt.Sprintf("%d.%d.%d.%d", blk[0], blk[1], blk[2], rand.Intn(256))
}

func randomIPv6() string {
	bases := []string{
		"2606:4700:d0::",
		"2606:4700:d1::",
	}
	base := bases[rand.Intn(len(bases))]
	return fmt.Sprintf("[%s%x:%x:%x:%x]", base,
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
		rand.Intn(0x10000),
	)
}

func tcpPing(ip, port string) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), timeout)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}

func scanIPs(gen func() string, label string) {
	fmt.Printf("\n%s ENDPOINTS (Best by lowest latency):\n", label)
	unique := map[string]bool{}
	var results []Endpoint
	for len(unique) < ipCount {
		ip := gen()
		if unique[ip] {
			continue
		}
		unique[ip] = true
		lat, err := tcpPing(ip, port)
		if err == nil {
			results = append(results, Endpoint{IP: ip, Latency: lat})
			fmt.Printf("✔ %s:%s -> %v\n", ip, port, lat)
		} else {
			fmt.Printf("✗ %s:%s -> timeout\n", ip, port)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})
	fmt.Println("\nTop results:")
	for i, r := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. %s:%s - %v\n", i+1, r.IP, port, r.Latency)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Choose scan type:")
		fmt.Println("1. Preferred IPv4")
		fmt.Println("2. Preferred IPv6")
		fmt.Println("3. Exit")
		fmt.Print("> ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		switch choice {
		case "1":
			scanIPs(randomIPv4, "IPv4")
		case "2":
			scanIPs(randomIPv6, "IPv6")
		case "3":
			fmt.Println("Exiting.")
			return
		default:
			fmt.Println("Invalid choice, try again.")
		}
		fmt.Println()
	}
}
