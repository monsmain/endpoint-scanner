package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type EndpointResult struct {
	Endpoint string
	Ping     string
	Loss     string
}

func generateIPv4Addresses() []string {
	var ips []string
	subnets := []string{
		"162.159.192.", "162.159.193.", "162.159.195.",
		"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
	}
	for _, subnet := range subnets {
		for i := 0; i < 50; i++ { 
			ips = append(ips, fmt.Sprintf("%s%d", subnet, rand.Intn(256)))
		}
	}
	return ips
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func downloadWarperEndpoint() error {
	var cpuArch string
	switch runtime.GOARCH {
	case "amd64":
		cpuArch = "amd64"
	case "386":
		cpuArch = "386"
	case "arm64":
		cpuArch = "arm64"
	case "arm":
		cpuArch = "arm"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/Ptechgithub/warp/main/endip/%s", cpuArch)
	fmt.Printf("Downloading 'warpendpoint' for %s architecture...\n", cpuArch)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	err = ioutil.WriteFile("warpendpoint", body, 0755) 
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Println("Download complete.")
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if !fileExists("warpendpoint") {
		err := downloadWarperEndpoint()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Generating IP list...")
	ips := generateIPv4Addresses()
	ipFile, err := os.Create("ip.txt")
	if err != nil {
		fmt.Printf("Error creating ip.txt: %v\n", err)
		os.Exit(1)
	}
	for _, ip := range ips {
		fmt.Fprintln(ipFile, ip)
	}
	ipFile.Close()

	fmt.Println("Running 'warpendpoint' to find the best endpoints... This may take a moment.")
	cmd := exec.Command("./warpendpoint")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error running warpendpoint: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nScan complete. Processing results...")

	resultFile, err := os.Open("result.csv")
	if err != nil {
		fmt.Printf("Error opening result.csv: %v\n", err)
		os.Exit(1)
	}
	defer resultFile.Close()

	var results []EndpointResult
	scanner := bufio.NewScanner(resultFile)
	isFirstLine := true
	for scanner.Scan() {
		if isFirstLine { 
			isFirstLine = false
			continue
		}
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) == 3 {
			results = append(results, EndpointResult{
				Endpoint: parts[0],
				Loss:     parts[1],
				Ping:     parts[2],
			})
		}
	}

	if len(results) == 0 {
		fmt.Println("No working endpoints found.")
		return
	}
    


	fmt.Println("\n--- Best Endpoint Found ---")
	bestEndpoint := results[0]
	fmt.Printf("🏆 Best Endpoint: %s\n", bestEndpoint.Endpoint)
	fmt.Printf("   Ping: %s\n", bestEndpoint.Ping)
    fmt.Printf("   Packet Loss: %s\n\n", bestEndpoint.Loss)
    fmt.Println("(You can now use this Endpoint in your app)")

	fmt.Println("\n--- Top 5 Endpoints ---")
	for i, result := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. Endpoint: %s (Ping: %s, Loss: %s)\n", i+1, result.Endpoint, result.Ping, result.Loss)
	}

	os.Remove("ip.txt")
	os.Remove("result.csv")
}
