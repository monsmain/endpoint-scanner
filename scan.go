package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

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
	// To increase the chance of finding the specific IP, we generate more addresses per subnet.
	for _, subnet := range subnets {
		for i := 0; i < 50; i++ { // Increased from 5 to 50
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	return ips
}

func generateIPv6Addresses() []string {
	var ips []string
	prefixes := []string{"2606:4700:d0::", "2606:4700:d1::"}
	for _, prefix := range prefixes {
		// Increased from 5 to 50
		for i := 0; i < 50; i++ {
			ip := fmt.Sprintf("%s%x:%x:%x:%x",
				prefix, rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff))
			ips = append(ips, ip)
		}
	}
	return ips
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

	fmt.Println("Step 1: Generating IP addresses to scan...")
	allIPs := append(generateIPv4Addresses(), generateIPv6Addresses()...)
	fmt.Printf("Generated %d unique IPs to test.\n", len(allIPs))

	fmt.Println("\nStep 2: Scanning all IPs for specific TCP and UDP ports...")
	fmt.Println("This will take some time. Please be patient.")
        // 19 tcp
	tcpPorts := []int{8886, 908, 8854, 4198, 955, 988, 3854, 894, 7156, 1074, 939, 864, 854, 1070, 3476, 1387, 7559, 890, 1018}
        // 6 udp
	udpPorts := []int{500, 1701, 4500, 2408, 878, 2371}

	var portWg sync.WaitGroup
	endpointResultsChan := make(chan EndpointResult, len(allIPs)*(len(tcpPorts)+len(udpPorts)))

	// Scan TCP ports on ALL generated IPs
	for _, ip := range allIPs {
		for _, port := range tcpPorts {
			portWg.Add(1)
			go scanPort(ip, port, "tcp", tcpTimeout, endpointResultsChan, &portWg)
		}
	}

	// Scan UDP ports on ALL generated IPs
	for _, ip := range allIPs {
		for _, port := range udpPorts {
			portWg.Add(1)
			go scanPort(ip, port, "udp", udpTimeout, endpointResultsChan, &portWg)
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
		fmt.Println("This may be due to heavy network restrictions or a configuration issue.")
		fmt.Println("-------------------------------------------------------------")
		return
	}

	fmt.Println("\n--- TCP Results ---")
	if len(tcpResults) > 0 {
		sort.Slice(tcpResults, func(i, j int) bool {
			return tcpResults[i].Latency < tcpResults[j].Latency
		})
		bestEndpoint := tcpResults[0]
		fmt.Printf("🏆 Best TCP Endpoint: %s\n", bestEndpoint.Endpoint)
		fmt.Printf("   Latency: %.2f ms\n\n", float64(bestEndpoint.Latency.Nanoseconds())/1e6)

		fmt.Println("--- Top 10 TCP Endpoints ---")
		for i, result := range tcpResults {
			if i >= 10 {
				break
			}
			fmt.Printf("%d. Endpoint: %s (Latency: %.2f ms)\n", i+1, result.Endpoint, float64(result.Latency.Nanoseconds())/1e6)
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
		fmt.Printf("🏆 Best UDP Endpoint: %s\n", bestEndpoint.Endpoint)
		fmt.Printf("   Latency: %.2f ms\n\n", float64(bestEndpoint.Latency.Nanoseconds())/1e6)

		fmt.Println("--- Top 10 UDP Endpoints ---")
		for i, result := range udpResults {
			if i >= 10 {
				break
			}
			fmt.Printf("%d. Endpoint: %s (Latency: %.2f ms)\n", i+1, result.Endpoint, float64(result.Latency.Nanoseconds())/1e6)
		}
	} else {
		fmt.Println("No open UDP Endpoints were found.")
	}
}
