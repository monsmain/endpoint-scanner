package main

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/go-ping/ping"
)

type PingResult struct {
	IP  string
	RTT time.Duration
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
	return ips
}

func generateIPv6Addresses() []string {
	var ips []string
	prefixes := []string{"2606:4700:d0::", "2606:4700:d1::"}
	for _, prefix := range prefixes {
		for i := 0; i < 10; i++ {
			ip := fmt.Sprintf("%s%x:%x:%x:%x",
				prefix, rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff), rand.Intn(0xffff))
			ips = append(ips, ip)
		}
	}
	return ips
}

func main() {
	rand.Seed(time.Now().UnixNano())

	allIPs := append(generateIPv4Addresses(), generateIPv6Addresses()...)
	fmt.Println("Starting to ping Cloudflare endpoints...")
	fmt.Println("NOTE: This program requires administrator/root privileges to run.")

	var wg sync.WaitGroup
	resultsChan := make(chan PingResult, len(allIPs))

	for _, ip := range allIPs {
		wg.Add(1)
		go func(ipAddr string) {
			defer wg.Done()
			pinger, err := ping.NewPinger(ipAddr)
			if err != nil {
				// Print error if the pinger cannot be created for an IP
				fmt.Printf("ERROR: Failed to create pinger for %s: %v\n", ipAddr, err)
				return
			}

			pinger.SetPrivileged(true)
			pinger.Count = 3
			pinger.Timeout = time.Second * 2

			pinger.OnFinish = func(stats *ping.Statistics) {
				if stats.PacketsRecv > 0 {
					resultsChan <- PingResult{IP: stats.Addr, RTT: stats.AvgRtt}
				} else {
					// Notify which IP did not respond
					fmt.Printf("INFO: No response from %s (Timeout or Host Unreachable)\n", stats.Addr)
				}
			}
			pinger.Run()
		}(ip)
	}

	wg.Wait()
	close(resultsChan)

	var results []PingResult
	for result := range resultsChan {
		results = append(results, result)
	}

	if len(results) == 0 {
		fmt.Println("\n-------------------------------------------------------------")
		fmt.Println("CRITICAL: No responsive IP addresses found.")
		fmt.Println("Please check the following:")
		fmt.Println("1. Did you run this program with 'sudo' (Linux/Mac) or as Administrator (Windows)?")
		fmt.Println("2. Is your internet connection working?")
		fmt.Println("3. Is a firewall blocking ICMP packets?")
		fmt.Println("-------------------------------------------------------------")
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RTT < results[j].RTT
	})

	fmt.Println("\n--- Top 5 Results ---")
	for i, result := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. IP: %s - Average Ping: %s\n", i+1, result.IP, result.RTT)
	}

	fmt.Println("\n--- Best Endpoints ---")
	
	var bestIPv4 PingResult
	for _, res := range results {
		if net.ParseIP(res.IP).To4() != nil {
			bestIPv4 = res
			break
		}
	}
	if bestIPv4.IP != "" {
		fmt.Printf("Preferred IPV4: %s with a ping of %s\n", bestIPv4.IP, bestIPv4.RTT)
	} else {
		fmt.Println("No suitable IPv4 address was found.")
	}

	var bestIPv6 PingResult
	for _, res := range results {
		if net.ParseIP(res.IP).To4() == nil {
			bestIPv6 = res
			break
		}
	}
	if bestIPv6.IP != "" {
		fmt.Printf("Preferred IPV6: %s with a ping of %s\n", bestIPv6.IP, bestIPv6.RTT)
	} else {
		fmt.Println("No suitable IPv6 address was found.")
	}
}
