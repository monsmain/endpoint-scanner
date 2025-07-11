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

func generateIPv6Addresses() []string {
	var ips []string
	prefixes := []string{"2606:4700:d0::", "2606:4700:d1::"}
	for _, prefix := range prefixes {
		for i := 0; i < 20; i++ { 
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

func main() {
	rand.Seed(time.Now().UnixNano())

	allIPs := append(generateIPv4Addresses(), generateIPv6Addresses()...)
	fmt.Println("Starting to ping Cloudflare endpoints using Termux's ping tool...")

	var wg sync.WaitGroup
	resultsChan := make(chan PingResult, len(allIPs))

	for _, ip := range allIPs {
		wg.Add(1)
		go func(ipAddr string) {
			defer wg.Done()
			rtt, err := pingWithTermux(ipAddr)
			if err == nil {
				resultsChan <- PingResult{IP: ipAddr, RTT: rtt}
			} else {
			}
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
		fmt.Println("Please check your internet connection or try again.")
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
