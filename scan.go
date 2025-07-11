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
	"sort"
	"strconv"
	"strings"
	"time"
)

type EndpointResult struct {
	Endpoint string
	Ping     float64
	Loss     int
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
	fmt.Printf("Downloading 'warpendpoint' tool for %s architecture...\n", cpuArch)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	err = ioutil.WriteFile("warpendpoint", body, 0755)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	fmt.Println("Download complete.")
	return nil
}

func displayProgressBar(done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			chars := []string{"|", "/", "-", "\\"}
			for _, char := range chars {
				fmt.Printf("\rScanning... %s", char)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
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

	done := make(chan bool)
	go displayProgressBar(done)

	cmd := exec.Command("./warpendpoint")
	cmd.Run()

	done <- true
	fmt.Print("\rScan complete.          \n")

	resultFile, err := os.Open("result.csv")
	if err != nil {
		fmt.Println("\nCould not find result.csv. It seems no working endpoints were found.")
		os.Exit(1)
	}
	defer resultFile.Close()

	var results []EndpointResult
	scanner := bufio.NewScanner(resultFile)
	scanner.Scan() // Skip header line
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) == 3 {
			pingStr := strings.TrimSpace(strings.Replace(parts[2], "ms", "", -1))
			ping, _ := strconv.ParseFloat(pingStr, 64)
			loss, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

			results = append(results, EndpointResult{
				Endpoint: parts[0],
				Loss:     loss,
				Ping:     ping,
			})
		}
	}

	if len(results) == 0 {
		fmt.Println("No working endpoints found.")
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Ping < results[j].Ping
	})

	fmt.Println("\n--- Best Endpoint Found ---")
	bestEndpoint := results[0]
	fmt.Printf("🏆 Best Endpoint: %s\n", bestEndpoint.Endpoint)
	fmt.Printf("   Ping: %.2f ms\n", bestEndpoint.Ping)
	fmt.Printf("   Packet Loss: %d%%\n\n", bestEndpoint.Loss)
	fmt.Println("(You can now use this Endpoint in your app)")

	fmt.Println("\n--- Top 5 Endpoints ---")
	for i, result := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. Endpoint: %s (Ping: %.2f ms, Loss: %d%%)\n", i+1, result.Endpoint, result.Ping, result.Loss)
	}

	os.Remove("ip.txt")
	os.Remove("result.csv")
}
