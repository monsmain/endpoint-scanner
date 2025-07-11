package main

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type EndpointResult struct {
	Endpoint string
	Ping     time.Duration
	Protocol string
}

func probeEndpoint(ip, protocol string, port int, resultsChan chan<- EndpointResult, wg *sync.WaitGroup, counter *uint64) {
	defer wg.Done()

	endpoint := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	
	start := time.Now()
	conn, err := net.DialTimeout(protocol, endpoint, 2*time.Second)
	if err != nil {
		atomic.AddUint64(counter, 1)
		return
	}
	defer conn.Close()
	ping := time.Since(start)

	if protocol == "udp" {
		handshakePacket := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		_, err = conn.Write(handshakePacket)
		if err != nil {
			atomic.AddUint64(counter, 1)
			return
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buffer := make([]byte, 1)
		_, err = conn.Read(buffer)
		if err != nil {
			atomic.AddUint64(counter, 1)
			return
		}
	}
    
	resultsChan <- EndpointResult{Endpoint: endpoint, Ping: ping, Protocol: protocol}
	atomic.AddUint64(counter, 1)
}

func main() {
	subnets := []string{
		"162.159.192", "162.159.193", "162.159.195",
		"188.114.96", "188.114.97", "188.114.98", "188.114.99",
	}
	
	portsToScan := []int{4500, 5956, 8886, 2408, 908, 1701, 500, 878, 443}
	protocolsToScan := []string{"tcp", "udp"}

	var ipsToScan []string
	for _, subnet := range subnets {
		for i := 0; i <= 255; i++ {
			ipsToScan = append(ipsToScan, fmt.Sprintf("%s.%d", subnet, i))
		}
	}

	totalJobs := len(ipsToScan) * len(portsToScan) * len(protocolsToScan)
	fmt.Printf("Starting a comprehensive scan of %d IPs across %d ports (%d total probes).\n", len(ipsToScan), len(portsToScan), totalJobs)
	fmt.Println("This will take a significant amount of time. Please be patient.")

	var wg sync.WaitGroup
	resultsChan := make(chan EndpointResult, 100)
	
	var progressCounter uint64

	concurrencyLimit := 200
	guard := make(chan struct{}, concurrencyLimit)

	for _, protocol := range protocolsToScan {
		for _, ip := range ipsToScan {
			for _, port := range portsToScan {
				wg.Add(1)
				guard <- struct{}{}

				go func(p, i string, pt int) {
					// This is the line that was fixed
					probeEndpoint(i, p, pt, resultsChan, &wg, &progressCounter)
					<-guard
				}(protocol, ip, port)
			}
		}
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			progress := atomic.LoadUint64(&progressCounter)
			if progress >= uint64(totalJobs) {
				break
			}
			percentage := float64(progress) / float64(totalJobs) * 100
			fmt.Printf("\rScanning... %.2f%% complete", percentage)
		}
	}()

	wg.Wait()
	close(resultsChan)
	
	fmt.Printf("\rScanning... 100.00%% complete\n")

	var udpResults []EndpointResult
	var tcpResults []EndpointResult

	for result := range resultsChan {
		if result.Protocol == "udp" {
			udpResults = append(udpResults, result)
		} else {
			tcpResults = append(tcpResults, result)
		}
	}

	sort.Slice(udpResults, func(i, j int) bool { return udpResults[i].Ping < udpResults[j].Ping })
	sort.Slice(tcpResults, func(i, j int) bool { return tcpResults[i].Ping < tcpResults[j].Ping })

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
