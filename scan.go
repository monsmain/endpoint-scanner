package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ScanJob struct {
	IP       string
	Port     int
	Protocol string
	Timeout  time.Duration
}

type EndpointResult struct {
	Endpoint string
	Latency  time.Duration
	Protocol string
}

func worker(jobs <-chan ScanJob, results chan<- EndpointResult, wg *sync.WaitGroup, completedJobs *uint64, debugMode bool) {
	defer wg.Done()
	for job := range jobs {
		defer atomic.AddUint64(completedJobs, 1)

		address := net.JoinHostPort(job.IP, strconv.Itoa(job.Port))
		start := time.Now()
		conn, err := net.DialTimeout(job.Protocol, address, job.Timeout)
		latency := time.Since(start)

		if err == nil {
			conn.Close()
			results <- EndpointResult{Endpoint: address, Latency: latency, Protocol: job.Protocol}
		} else if debugMode && job.Protocol == "tcp" {
			fmt.Printf("\n[DEBUG] TCP Connect Error for %s: %v", address, err)
		}
	}
}

func printProgress(completedJobs *uint64, totalJobs int, done chan bool) {
	startTime := time.Now()
	for {
		select {
		case <-done:
			fmt.Println("\nProgress: 100% | Scan Complete.")
			return
		default:
			completed := atomic.LoadUint64(completedJobs)
			percent := float64(completed) / float64(totalJobs) * 100
			
			elapsed := time.Since(startTime).Seconds()
			var eta float64
			if completed > 0 {
				eta = (elapsed / float64(completed)) * float64(totalJobs-int(completed))
			}

			bar := strings.Repeat("=", int(percent/2)) + strings.Repeat(" ", 50-int(percent/2))
			fmt.Printf("\rProgress: [%s] %.2f%% | ETA: %.0fs", bar, percent, eta)
			time.Sleep(200 * time.Millisecond)
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
		for i := 0; i < 25; i++ {
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	return ips
}

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

func main() {
	tcpTimeout := 5 * time.Second
	udpTimeout := 3 * time.Second
	numWorkers := 100
	debugMode  := true 

	rand.Seed(time.Now().UnixNano())

	fmt.Println("Step 1: Generating IP addresses to scan...")
	allIPs := append(generateIPv4Addresses(), generateIPv6Addresses()...)
	allIPs = append(allIPs, "188.114.98.224") 

	fmt.Printf("Generated %d IPs to test.\n", len(allIPs))
	fmt.Println("\nStep 2: Starting scanner workers and queuing jobs...")

	tcpPorts := []int{8886, 908, 8854, 4198, 955, 988, 3854, 894, 7156, 1074, 939, 864, 854, 1070, 3476, 1387, 7559, 890, 1018}
	udpPorts := []int{500, 1701, 4500, 2408, 878, 2371}
	
	totalJobs := (len(allIPs) * len(tcpPorts)) + (len(allIPs) * len(udpPorts))
	fmt.Printf("Queuing %d total scan jobs for %d workers...\n", totalJobs, numWorkers)

	jobs := make(chan ScanJob, totalJobs)
	results := make(chan EndpointResult, totalJobs)
	var completedJobs uint64
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg, &completedJobs, debugMode)
	}
	
	doneProgress := make(chan bool)
	go printProgress(&completedJobs, totalJobs, doneProgress)

	for _, ip := range allIPs {
		for _, port := range tcpPorts {
			jobs <- ScanJob{IP: ip, Port: port, Protocol: "tcp", Timeout: tcpTimeout}
		}
		for _, port := range udpPorts {
			jobs <- ScanJob{IP: ip, Port: port, Protocol: "udp", Timeout: udpTimeout}
		}
	}
	close(jobs)

	wg.Wait()
	doneProgress <- true
	
	close(results)

	fmt.Println("\nScan complete. Processing results...")

	var tcpResults []EndpointResult
	var udpResults []EndpointResult

	for result := range results {
		if result.Protocol == "tcp" {
			tcpResults = append(tcpResults, result)
		} else {
			udpResults = append(udpResults, result)
		}
	}

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
