package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"
)

type EndpointResult struct {
	Endpoint string
	Ping     time.Duration
}

func generateIPv4Addresses() []string {
	var ips []string
	subnets := []string{
		"162.159.192.", "162.159.193.", "162.159.195.",
		"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
	}
	for _, subnet := range subnets {
		for i := 0; i < 20; i++ {
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	return ips
}


func probeEndpoint(ip string, port int, resultsChan chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()

	endpoint := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("udp", endpoint, 2*time.Second)
	if err != nil {
		return 
	}
	defer conn.Close()

	handshakePacket := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	start := time.Now()
	_, err = conn.Write(handshakePacket)
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))


	buffer := make([]byte, 1)
	_, err = conn.Read(buffer)
	ping := time.Since(start)

	if err == nil {
		resultsChan <- EndpointResult{Endpoint: endpoint, Ping: ping}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("Generating IP list...")
	ips := generateIPv4Addresses()

	fmt.Println("Probing endpoints with custom handshake... This may take a moment.")

	portsToScan := []int{2408, 500, 1701, 4500, 8886, 908, 8854, 878}

	var wg sync.WaitGroup
	resultsChan := make(chan EndpointResult, len(ips)*len(portsToScan))

	concurrencyLimit := 100
	guard := make(chan struct{}, concurrencyLimit)

	for _, ip := range ips {
		for _, port := range portsToScan {
			wg.Add(1)
			guard <- struct{}{} 

			go func(ip string, port int) {
				probeEndpoint(ip, port, resultsChan, &wg)
				<-guard 
			}(ip, port)
		}
	}

	wg.Wait()
	close(resultsChan)

	var results []EndpointResult
	for result := range resultsChan {
		results = append(results, result)
	}

	if len(results) == 0 {
		fmt.Println("\nCRITICAL: Could not find any working endpoints.")
		fmt.Println("This might be due to network restrictions or an unlucky IP batch. Try running again.")
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Ping < results[j].Ping
	})

	fmt.Println("\n--- Best Endpoint Found ---")
	bestEndpoint := results[0]

	fmt.Printf("🏆 Best Endpoint: %s\n", bestEndpoint.Endpoint)
	fmt.Printf("   Real Ping: %.2f ms\n\n", float64(bestEndpoint.Ping.Nanoseconds())/1e6)
	fmt.Println("(The lower the ms, the faster the ping)")

	fmt.Println("\n--- Top 5 Endpoints ---")
	for i, result := range results {
		if i >= 5 {
			break
		}
		pingInMS := float64(result.Ping.Nanoseconds()) / 1e6
		fmt.Printf("%d. Endpoint: %s (Ping: %.2f ms)\n", i+1, result.Endpoint, pingInMS)
	}
}
