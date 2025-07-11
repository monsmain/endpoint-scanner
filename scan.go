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
	Protocol string
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

func probeEndpoint(ip, protocol string, port int, resultsChan chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()

	endpoint := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	
	start := time.Now()
	conn, err := net.DialTimeout(protocol, endpoint, 2*time.Second)
	if err != nil {
		return 
	}
	defer conn.Close()
    ping := time.Since(start)

	if protocol == "udp" {
		handshakePacket := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		_, err = conn.Write(handshakePacket)
		if err != nil {
			return
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buffer := make([]byte, 1)
		_, err = conn.Read(buffer)
		if err != nil {
			return
		}
	}
    
	resultsChan <- EndpointResult{Endpoint: endpoint, Ping: ping, Protocol: protocol}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("Generating IP list...")
	ips := generateIPv4Addresses()

	fmt.Println("Probing TCP and UDP endpoints... This may take a moment.")

	portsToScan := []int{2408, 500, 1701, 4500, 8886, 908, 8854, 878, 443, 80, 8080}
	protocolsToScan := []string{"udp", "tcp"}

	var wg sync.WaitGroup
	resultsChan := make(chan EndpointResult, len(ips)*len(portsToScan)*len(protocolsToScan))

	concurrencyLimit := 100
	guard := make(chan struct{}, concurrencyLimit)

	for _, protocol := range protocolsToScan {
		for _, ip := range ips {
			for _, port := range portsToScan {
				wg.Add(1)
				guard <- struct{}{} 

				go func(p, i string, pt int) {
					probeEndpoint(i, p, pt, resultsChan, &wg)
					<-guard 
				}(protocol, ip, port)
			}
		}
	}

	wg.Wait()
	close(resultsChan)

	var udpResults []EndpointResult
	var tcpResults []EndpointResult

	for result := range resultsChan {
		if result.Protocol == "udp" {
			udpResults = append(udpResults, result)
		} else {
			tcpResults = append(tcpResults, result)
		}
	}

	sort.Slice(udpResults, func(i, j int) bool {
		return udpResults[i].Ping < udpResults[j].Ping
	})
	sort.Slice(tcpResults, func(i, j int) bool {
		return tcpResults[i].Ping < tcpResults[j].Ping
	})

	fmt.Println("\n--- Scan Complete! ---")

	if len(udpResults) > 0 {
		fmt.Println("\n--- Best UDP Endpoint ---")
		bestUDP := udpResults[0]
		pingInMS := float64(bestUDP.Ping.Nanoseconds()) / 1e6
		fmt.Printf("🏆 Endpoint: %s (UDP)\n", bestUDP.Endpoint)
		fmt.Printf("   Ping: %.2f ms\n", pingInMS)
	} else {
		fmt.Println("\nNo working UDP endpoints found.")
	}

	if len(tcpResults) > 0 {
		fmt.Println("\n--- Best TCP Endpoint ---")
		bestTCP := tcpResults[0]
		pingInMS := float64(bestTCP.Ping.Nanoseconds()) / 1e6
		fmt.Printf("🏆 Endpoint: %s (TCP)\n", bestTCP.Endpoint)
		fmt.Printf("   Ping: %.2f ms\n", pingInMS)
	} else {
		fmt.Println("\nNo working TCP endpoints found.")
	}
    
    fmt.Println("\n(The lower the ms, the faster the ping)")
}
