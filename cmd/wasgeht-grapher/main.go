package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kylerisse/wasgeht/pkg/rrd"
)

type HostData struct {
	Latency    float64 `json:"latency"`
	LastUpdate int64   `json:"lastupdate"`
}

func main() {
	// Create a new RRD file for example2
	fmt.Println("Initializing RRD file for host 'example2'...")
	rrd2File, err := rrd.NewRRD("example2", "./rrds")
	if err != nil {
		fmt.Println("Failed to create RRD:", err)
		return
	}
	fmt.Println("RRD file successfully initialized.")

	// Create a new RRD file for example3
	fmt.Println("Initializing RRD file for host 'example3'...")
	rrd3File, err := rrd.NewRRD("example3", "./rrds")
	if err != nil {
		fmt.Println("Failed to create RRD:", err)
		return
	}
	fmt.Println("RRD file successfully initialized.")

	// Run the loop every 30 seconds and stop after 5 minutes
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	timeout := time.After(168 * time.Hour)
	fmt.Println("Starting monitoring loop, running every 30 seconds for 7 days...")

	for {
		select {
		case <-timeout:
			fmt.Println("7 day monitoring period has ended. Exiting.")
			return
		case <-ticker.C:
			fmt.Println("Attempting to fetch data from API...")

			// Fetch data from the API
			host2Data, err := fetchHostData("http://localhost:1982/api", "router")
			if err != nil {
				fmt.Printf("Failed to fetch host data: %v\n", err)
				continue
			}

			fmt.Printf("Fetched data - Latency: %f microseconds, Last Update: %d\n", host2Data.Latency, host2Data.LastUpdate)

			// Update the RRD file with the fetched latency and timestamp
			fmt.Println("Updating RRD file with the fetched data...")
			err = rrd2File.SafeUpdate(time.Unix(host2Data.LastUpdate, 0), []float64{host2Data.Latency})
			if err != nil {
				fmt.Printf("Failed to update RRD: %v\n", err)
			} else {
				fmt.Println("RRD updated successfully with latency:", host2Data.Latency)
			}

			fmt.Println("Attempting to fetch data from API...")

			// Fetch data from the API
			host3Data, err := fetchHostData("http://localhost:1982/api", "onedotone")
			if err != nil {
				fmt.Printf("Failed to fetch host data: %v\n", err)
				continue
			}

			fmt.Printf("Fetched data - Latency: %f microseconds, Last Update: %d\n", host3Data.Latency, host3Data.LastUpdate)

			// Update the RRD file with the fetched latency and timestamp
			fmt.Println("Updating RRD file with the fetched data...")
			err = rrd3File.SafeUpdate(time.Unix(host3Data.LastUpdate, 0), []float64{host3Data.Latency})
			if err != nil {
				fmt.Printf("Failed to update RRD: %v\n", err)
			} else {
				fmt.Println("RRD updated successfully with latency:", host3Data.Latency)
			}
		}
	}
}

// fetchHostData fetches latency and lastupdate from the given API URL for the specified host.
func fetchHostData(apiURL string, hostName string) (HostData, error) {
	var hostData HostData

	fmt.Println("Making HTTP GET request to API:", apiURL)

	// Make the HTTP GET request
	resp, err := http.Get(apiURL)
	if err != nil {
		return hostData, fmt.Errorf("failed to make GET request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("HTTP GET request successful. Reading response body...")

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return hostData, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Println("Response body successfully read. Parsing JSON...")

	// Parse the JSON response
	var data map[string]HostData
	if err := json.Unmarshal(body, &data); err != nil {
		return hostData, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Extract the host data for the specified hostName
	hostData, exists := data[hostName]
	if !exists {
		return hostData, fmt.Errorf("host %s not found in response", hostName)
	}

	fmt.Printf("Host '%s' data extracted from response.\n", hostName)

	return hostData, nil
}
