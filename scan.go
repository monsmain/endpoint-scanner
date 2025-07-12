package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PingResult struct {
	IP  string
	RTT time.Duration
}

type EndpointResult struct {
	Endpoint string
	Latency  time.Duration
	Protocol string
}

func generateIPv4Addresses() []string {
	var ips []string
	subnets := []string{
		"162.159.192.", "162.159.193.", "162.159.195.",
		"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
	}
	for _, subnet := range subnets {
		for i := 0; i < 5; i++ {
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	return ips
}

func generateIPv6Addresses() []string {
	var ips []string
	prefixes := []string{"2606:4700:d0::", "2606:4700:d1::"}
	for _, prefix := range prefixes {
		for i := 0; i < 5; i++ {
			ip := fmt.Sprintf("%s%x:%x:%x:%x",
				prefix, rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff))
			ips = append(ips, ip)
		}
	}
	return ips
}

func pingWithTermux(ipAddr string) (time.Duration, error) {
	cmd := exec.Command("ping", "-c", "3", "-W", "2", ipAddr)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(stdout)
	var avgRtt time.Duration
	rttRegex := regexp.MustCompile(`rtt min/avg/max/mdev = [\d.]+/([\d.]+)/[\d.]+/[\d.]+ ms`)
	for scanner.Scan() {
		line := scanner.Text()
		matches := rttRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			avg, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				avgRtt = time.Duration(avg * float64(time.Millisecond))
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return 0, fmt.Errorf("no response from host")
	}
	if avgRtt == 0 {
		return 0, fmt.Errorf("could not parse RTT")
	}
	return avgRtt, nil
}

func scanPort(ip string, port int, protocol string, resultsChan chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	start := time.Now()
	conn, err := net.DialTimeout(protocol, address, 1*time.Second)
	latency := time.Since(start)

	if err == nil {
		conn.Close()
		resultsChan <- EndpointResult{Endpoint: address, Latency: latency, Protocol: protocol}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("Step 1: Finding best IPs with ping...")
	allIPs := append(generateIPv4Addresses(), generateIPv6Addresses()...)

	var pingWg sync.WaitGroup
	pingResultsChan := make(chan PingResult, len(allIPs))

	for _, ip := range allIPs {
		pingWg.Add(1)
		go func(ipAddr string) {
			defer pingWg.Done()
			rtt, err := pingWithTermux(ipAddr)
			if err == nil {
				pingResultsChan <- PingResult{IP: ipAddr, RTT: rtt}
			}
		}(ip)
	}

	pingWg.Wait()
	close(pingResultsChan)

	var bestIPs []PingResult
	for result := range pingResultsChan {
		bestIPs = append(bestIPs, result)
	}

	if len(bestIPs) == 0 {
		fmt.Println("No responsive IPs found in Step 1. Exiting.")
		return
	}

	sort.Slice(bestIPs, func(i, j int) bool {
		return bestIPs[i].RTT < bestIPs[j].RTT
	})

	ipToPing := make(map[string]time.Duration)
	for _, ipResult := range bestIPs {
		ipToPing[ipResult.IP] = ipResult.RTT
	}

	fmt.Println("Step 1 Complete. Best IPs found.")
	fmt.Println("\nStep 2: Scanning ports on best IPs to find a full endpoint (TCP & UDP)...")

	portsToScan := []int{2408, 500, 1701, 4500, 8886, 908, 8854, 878}
	protocolsToScan := []string{"udp", "tcp"}

	var portWg sync.WaitGroup
	endpointResultsChan := make(chan EndpointResult, len(bestIPs)*len(portsToScan)*len(protocolsToScan))

	scanLimit := 10
	if len(bestIPs) < scanLimit {
		scanLimit = len(bestIPs)
	}

	for _, ipResult := range bestIPs[:scanLimit] {
		for _, port := range portsToScan {
			for _, proto := range protocolsToScan {
				portWg.Add(1)
				go scanPort(ipResult.IP, port, proto, endpointResultsChan, &portWg)
			}
		}
	}

	portWg.Wait()
	close(endpointResultsChan)

	var tcpResults []EndpointResult
	var udpResults []EndpointResult

	for result := range endpointResultsChan {
		if result.Protocol == "tcp" {
			tcpResults = append(tcpResults, result)
		} else {
			udpResults = append(udpResults, result)
		}
	}

	if len(tcpResults) == 0 && len(udpResults) == 0 {
		fmt.Println("\n-------------------------------------------------------------")
		fmt.Println("CRITICAL: Could not find any open ports on the responsive IPs.")
		fmt.Println("This might be due to network restrictions.")
		fmt.Println("-------------------------------------------------------------")
		return
	}

	if len(tcpResults) > 0 {
		sort.Slice(tcpResults, func(i, j int) bool {
			return tcpResults[i].Latency < tcpResults[j].Latency
		})
		fmt.Println("\n--- Best TCP Endpoint Found ---")
		bestEndpoint := tcpResults[0]
		host, _, _ := net.SplitHostPort(bestEndpoint.Endpoint)
		realPing := ipToPing[host]
		fmt.Printf("🏆 Best Endpoint: %s (TCP)\n", bestEndpoint.Endpoint)
		fmt.Printf("   Real Ping: %.2f ms\n\n", float64(realPing.Nanoseconds())/1e6)

		fmt.Println("--- Top 5 TCP Endpoints ---")
		for i, result := range tcpResults {
			if i >= 5 {
				break
			}
			host, _, _ := net.SplitHostPort(result.Endpoint)
			realPing := ipToPing[host]
			fmt.Printf("%d. Endpoint: %s (Ping: %.2f ms)\n", i+1, result.Endpoint, float64(realPing.Nanoseconds())/1e6)
		}
	} else {
		fmt.Println("\n--- No open TCP Endpoints found ---")
	}

	if len(udpResults) > 0 {
		sort.Slice(udpResults, func(i, j int) bool {
			return udpResults[i].Latency < udpResults[j].Latency
		})
		fmt.Println("\n--- Best UDP Endpoint Found ---")
		bestEndpoint := udpResults[0]
		host, _, _ := net.SplitHostPort(bestEndpoint.Endpoint)
		realPing := ipToPing[host]
		fmt.Printf("🏆 Best Endpoint: %s (UDP)\n", bestEndpoint.Endpoint)
		fmt.Printf("   Real Ping: %.2f ms\n\n", float64(realPing.Nanoseconds())/1e6)

		fmt.Println("--- Top 5 UDP Endpoints ---")
		for i, result := range udpResults {
			if i >= 5 {
				break
			}
			host, _, _ := net.SplitHostPort(result.Endpoint)
			realPing := ipToPing[host]
			fmt.Printf("%d. Endpoint: %s (Ping: %.2f ms)\n", i+1, result.Endpoint, float64(realPing.Nanoseconds())/1e6)
		}
	} else {
		fmt.Println("\n--- No open UDP Endpoints found ---")
	}

	fmt.Println("\n(The lower the ms, the faster the ping)")
}
