package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScanJob represents a single port scan task.
type ScanJob struct {
	IP       string
	Port     int
	Protocol string
	Timeout  time.Duration
}

// EndpointResult holds the outcome of a successful scan.
type EndpointResult struct {
	Endpoint string
	Latency  time.Duration
	Protocol string
}

// worker processes jobs from the jobs channel.
func worker(id int, jobs <-chan ScanJob, results chan<- EndpointResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		address := net.JoinHostPort(job.IP, strconv.Itoa(job.Port))
		start := time.Now()
		conn, err := net.DialTimeout(job.Protocol, address, job.Timeout)
		latency := time.Since(start)

		if err == nil {
			conn.Close()
			results <- EndpointResult{Endpoint: address, Latency: latency, Protocol: job.Protocol}
		}
	}
}

func generateIPv4Addresses() []string {
	var ips []string
	subnets := []string{
		"162.159.192.", "162.159.193.", "162.159.195.",
		"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
	}
	for _, subnet := range subnets {
		// A balanced number of IPs for speed and coverage.
		for i := 0; i < 50; i++ {
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

/*
func generateIPv6Addresses() []string {
	var ips []string
	prefixes := []string{"2606:4700:d0::", "2606:4700:d1::"}
	for _, prefix := range prefixes {
		for i := 0; i < 25; i++ {
			ip := fmt.Sprintf("%s%x:%x:%x:%x",
				prefix, rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff))
			ips = append(ips, ip)
		}
	}
	return ips
}
*/

func main() {
	// --- Configuration ---
	tcpTimeout := 7 * time.Second // Increased timeout
	// udpTimeout := 3 * time.Second
	numWorkers := 20 // A controlled number of workers for balance
	// --- End Configuration ---

	rand.Seed(time.Now().UnixNano())

	fmt.Println("Step 1: Generating IPv4 addresses to scan...")
	allIPs := generateIPv4Addresses()
	// allIPs = append(allIPs, generateIPv6Addresses()...)
	allIPs = append(allIPs, "188.114.98.224") // Guaranteed IP Test

	fmt.Printf("Generated %d unique IPs to test.\n", len(allIPs))
	fmt.Println("\nStep 2: Starting balanced scan...")

	tcpPorts := []int{8886}
	// udpPorts := []int{500, 1701, 4500, 2408, 878, 2371}

	jobs := make(chan ScanJob)
	results := make(chan EndpointResult)
	var wg sync.WaitGroup

	// Start workers
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, jobs, results, &wg)
	}

	// Collect results in a separate goroutine
	var tcpResults []EndpointResult
	var udpResults []EndpointResult
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for result := range results {
			if result.Protocol == "tcp" {
				tcpResults = append(tcpResults, result)
			} else {
				udpResults = append(udpResults, result)
			}
		}
	}()

	// Create a list of all jobs
	allJobs := []ScanJob{}
	for _, ip := range allIPs {
		for _, port := range tcpPorts {
			allJobs = append(allJobs, ScanJob{IP: ip, Port: port, Protocol: "tcp", Timeout: tcpTimeout})
		}
		/*
			for _, port := range udpPorts {
				allJobs = append(allJobs, ScanJob{IP: ip, Port: port, Protocol: "udp", Timeout: udpTimeout})
			}
		*/
	}

	totalJobs := len(allJobs)
	fmt.Printf("Queuing %d total scan jobs for %d workers...\n", totalJobs, numWorkers)
	startTime := time.Now()

	// Feed jobs to the workers and update progress
	for i, job := range allJobs {
		jobs <- job
		percent := float64(i+1) / float64(totalJobs) * 100
		elapsed := time.Since(startTime).Seconds()
		jobsPerSecond := 0.0
		if elapsed > 0 {
			jobsPerSecond = float64(i+1) / elapsed
		}
		bar := strings.Repeat("=", int(percent/2)) + strings.Repeat(" ", 50-int(percent/2))
		fmt.Printf("\rProgress: [%s] %.2f%% | %.1f jobs/sec", bar, percent, jobsPerSecond)
	}
	close(jobs)

	wg.Wait()
	close(results)
	collectorWg.Wait()

	fmt.Println("\n\nScan complete. Processing results...")

	if len(tcpResults) == 0 && len(udpResults) == 0 {
		fmt.Println("\nCRITICAL: Could not find any open TCP or UDP ports.")
		return
	}

	fmt.Println("\n--- TCP Results ---")
	if len(tcpResults) > 0 {
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
	} else {
		fmt.Println("No open TCP Endpoints were found.")
	}

	fmt.Println("\n--- UDP Results ---")
	if len(udpResults) > 0 {
		sort.Slice(udpResults, func(i, j int) bool {
			return udpResults[i].Latency < udpResults[j].Latency
		})
		bestEndpoint := udpResults[0]
		fmt.Printf("🏆 Best UDP Endpoint: %s (Latency: %.2f ms)\n\n", bestEndpoint.Endpoint, float64(bestEndpoint.Latency.Nanoseconds())/1e6)

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
