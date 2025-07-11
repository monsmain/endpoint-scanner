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

func scanPort(ip string, port int, resultsChan chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()
	protocol := "udp"
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	start := time.Now()
	conn, err := net.DialTimeout(protocol, address, 1*time.Second)
	latency := time.Since(start)

	if err == nil {
		conn.Close()
		resultsChan <- EndpointResult{Endpoint: address, Latency: latency}
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

	fmt.Println("Step 1 Complete. Best IPs found.")

	fmt.Println("\nStep 2: Scanning ports on best IPs to find a full endpoint...")

	portsToScan := []int{2408, 500, 1701, 4500, 8886, 908, 8854, 878}

	var portWg sync.WaitGroup
	endpointResultsChan := make(chan EndpointResult, len(bestIPs)*len(portsToScan))

	scanLimit := 10
	if len(bestIPs) < scanLimit {
		scanLimit = len(bestIPs)
	}

	for _, ipResult := range bestIPs[:scanLimit] {
		for _, port := range portsToScan {
			portWg.Add(1)
			go scanPort(ipResult.IP, port, endpointResultsChan, &portWg)
		}
	}

	portWg.Wait()
	close(endpointResultsChan)

	var finalResults []EndpointResult
	for result := range endpointResultsChan {
		finalResults = append(finalResults, result)
	}

	if len(finalResults) == 0 {
		fmt.Println("\n-------------------------------------------------------------")
		fmt.Println("CRITICAL: Could not find any open ports on the responsive IPs.")
		fmt.Println("This might be due to network restrictions.")
		fmt.Println("-------------------------------------------------------------")
		return
	}

	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].Latency < finalResults[j].Latency
	})

	fmt.Println("\n--- Best Endpoint Found ---")
	bestEndpoint := finalResults[0]
	latencyInMS := float64(bestEndpoint.Latency.Nanoseconds()) / 1e6

	fmt.Printf("🏆 Best Endpoint: %s\n", bestEndpoint.Endpoint)
	fmt.Printf("   Connection Speed: %.2f ms\n\n", latencyInMS)
	fmt.Println("(The lower the ms, the faster the connection)")

	fmt.Println("\n--- Top 5 Endpoints ---")
	for i, result := range finalResults {
		if i >= 5 {
			break
		}
		latencyInMS := float64(result.Latency.Nanoseconds()) / 1e6
		fmt.Printf("%d. Endpoint: %s (Speed: %.2f ms)\n", i+1, result.Endpoint, latencyInMS)
	}
}
