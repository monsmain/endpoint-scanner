package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strings"
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
	for _, subnet := range subnets {
		for i := 0; i < 10; i++ {
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	// Remove duplicates
	ipMap := make(map[string]bool)
	var uniqueIPs []string
	for _, ip := range ips {
		if !ipMap[ip] {
			ipMap[ip] = true
			uniqueIPs = append(uniqueIPs, ip)
		}
	}
	return uniqueIPs
}

func main() {
	// --- Configuration ---
	tcpTimeout := 8 * time.Second
	scanDelay := 50 * time.Millisecond
	// --- End Configuration ---

	rand.Seed(time.Now().UnixNano())

	fmt.Println("Step 1: Generating IPv4 addresses to scan...")
	allIPs := generateIPv4Addresses()
	allIPs = append([]string{"188.114.98.224"}, allIPs...)

	fmt.Printf("Generated %d unique IPs to test.\n", len(allIPs))
	fmt.Println("\nStep 2: Starting slow, sequential scan...")

	tcpPorts := []int{8886, 908, 8854, 4198, 955, 988, 3854, 894, 7156, 1074, 939, 864, 854, 1070, 3476, 1387, 7559, 890, 1018}
	
	var allJobs []string
	for _, ip := range allIPs {
		for _, port := range tcpPorts {
			allJobs = append(allJobs, fmt.Sprintf("%s:%d", ip, port))
		}
	}
	
	totalJobs := len(allJobs)
	var tcpResults []EndpointResult

	startTime := time.Now()
	for i, job := range allJobs {
		// Update Progress Bar
		percent := float64(i+1) / float64(totalJobs) * 100
		elapsed := time.Since(startTime).Seconds()
		jobsPerSecond := float64(i+1) / elapsed
		bar := strings.Repeat("=", int(percent/2)) + strings.Repeat(" ", 50-int(percent/2))
		fmt.Printf("\rProgress: [%s] %.2f%% | %.1f jobs/sec | Testing: %-21s", bar, percent, jobsPerSecond, job)

		// Perform the scan
		startScan := time.Now()
		conn, err := net.DialTimeout("tcp", job, tcpTimeout)
		latency := time.Since(startScan)

		if err == nil {
			conn.Close()
			tcpResults = append(tcpResults, EndpointResult{Endpoint: job, Latency: latency, Protocol: "tcp"})
		}

		time.Sleep(scanDelay)
	}

	fmt.Println("\n\nScan complete. Processing results...")

	if len(tcpResults) == 0 {
		fmt.Println("\nCRITICAL: Could not find any open TCP ports.")
		return
	}

	fmt.Println("\n--- TCP Results ---")
	sort.Slice(tcpResults, func(i, j int) bool {
		return tcpResults[i].Latency < tcpResults[j].Latency
	})
	bestEndpoint := tcpResults[0]
	fmt.Printf("🏆 Best TCP Endpoint: %s (Latency: %.2f ms)\n\n", bestEndpoint.Endpoint, float64(bestEndpoint.Latency.Nanoseconds())/1e6)

	fmt.Println("--- Top 10 TCP Endpoints ---")
	for i, result := range tcpResults {
		if i >= 10 {
			break
		}
		fmt.Printf("%d. Endpoint: %s (Latency: %.2f ms)\n", i+1, result.Endpoint, float64(result.Latency.Nanoseconds())/1e6)
	}
}
