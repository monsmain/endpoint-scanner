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

func scanPort(ip string, port int, protocol string, timeout time.Duration, resultsChan chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	start := time.Now()
	conn, err := net.DialTimeout(protocol, address, timeout)
	latency := time.Since(start)

	if err == nil {
		conn.Close()
		resultsChan <- EndpointResult{Endpoint: address, Latency: latency, Protocol: protocol}
	}
}

func main() {

	tcpTimeout := 5 * time.Second
	udpTimeout := 5 * time.Second

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
	fmt.Println("\nStep 2: Scanning ports on best IPs (TCP & UDP)...")

	portsToScan := []int{443, 2408, 500, 1701, 4500, 8886, 908, 8854, 878, 4198, 955, 988, 3854, 894, 7156, 1074, 2371, 939, 864, 854, 1070, 3476, 1387, 7559, 890, 1018}
	var portWg sync.WaitGroup
	endpointResultsChan := make(chan EndpointResult, len(bestIPs)*len(portsToScan)*2)

	scanLimit := 30
	if len(bestIPs) < scanLimit {
		scanLimit = len(bestIPs)
	}

	for _, ipResult := range bestIPs[:scanLimit] {
		for _, port := range portsToScan {
			portWg.Add(2)
			go scanPort(ipResult.IP, port, "tcp", tcpTimeout, endpointResultsChan, &portWg)
			go scanPort(ipResult.IP, port, "udp", udpTimeout, endpointResultsChan, &portWg)
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
		fmt.Println("CRITICAL: Could not find any open TCP or UDP ports.")
		fmt.Println("This may be due to heavy network restrictions.")
		fmt.Println("-------------------------------------------------------------")
		return
	}

	fmt.Println("\n--- TCP Results ---")
	if len(tcpResults) > 0 {
		sort.Slice(tcpResults, func(i, j int) bool {
			return tcpResults[i].Latency < tcpResults[j].Latency
		})
		bestEndpoint := tcpResults[0]
		host, _, _ := net.SplitHostPort(bestEndpoint.Endpoint)
		realPing := ipToPing[host]
		fmt.Printf("🏆 Best TCP Endpoint: %s\n", bestEndpoint.Endpoint)
		fmt.Printf("   Latency: %.2f ms (Real Ping: %.2f ms)\n\n", float64(bestEndpoint.Latency.Nanoseconds())/1e6, float64(realPing.Nanoseconds())/1e6)

		fmt.Println("--- Top 10 TCP Endpoints ---")
		for i, result := range tcpResults {
			if i >= 10 {
				break
			}
			host, _, _ := net.SplitHostPort(result.Endpoint)
			realPing := ipToPing[host]
			fmt.Printf("%d. Endpoint: %s (Latency: %.2f ms, Real Ping: %.2f ms)\n", i+1, result.Endpoint, float64(result.Latency.Nanoseconds())/1e6, float64(realPing.Nanoseconds())/1e6)
		}
	} else {
		fmt.Println("No open TCP Endpoints were found.")
	}

	fmt.Println("\n--- UDP Results ---")
	if len(udpResults) > 0 {
		sort.Slice(udpResults, func(i, j int) bool {
			return udpResults[i].Latency < udpResults[j].Latency
		})
		bestEndpoint := udpResults[0]
		host, _, _ := net.SplitHostPort(bestEndpoint.Endpoint)
		realPing := ipToPing[host]
		fmt.Printf("🏆 Best UDP Endpoint: %s\n", bestEndpoint.Endpoint)
		fmt.Printf("   Latency: %.2f ms (Real Ping: %.2f ms)\n\n", float64(bestEndpoint.Latency.Nanoseconds())/1e6, float64(realPing.Nanoseconds())/1e6)

		fmt.Println("--- Top 10 UDP Endpoints ---")
		for i, result := range udpResults {
			if i >= 10 {
				break
			}
			host, _, _ := net.SplitHostPort(result.Endpoint)
			realPing := ipToPing[host]
			fmt.Printf("%d. Endpoint: %s (Latency: %.2f ms, Real Ping: %.2f ms)\n", i+1, result.Endpoint, float64(result.Latency.Nanoseconds())/1e6, float64(realPing.Nanoseconds())/1e6)
		}
	} else {
		fmt.Println("No open UDP Endpoints were found.")
	}
	fmt.Println("\n(Latency is the connection time to the port. Real Ping is the ICMP echo time to the IP.)")
}
