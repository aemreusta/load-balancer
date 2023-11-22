package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	ListenAddr        string   `json:"listenAddr"`
	Server            []string `json:"server"`
	ConnectionTimeout int      `json:"connectionTimeout"`
}

var (
	DefaultListenAddr        = "localhost:8080"
	DefaultServers           = [...]string{"localhost:5001", "localhost:5002", "localhost:5003"}
	DefaultConnectionTimeout = 60
	ConfigFile               = "config.json"
)

func main() {
	// Load configuration from the JSON file
	config, err := loadConfig(ConfigFile)
	if err != nil {
		log.Fatalf("failed to load configuration: %s", err)
	}

	// Create a TCP listener on the specified address or use the default
	listener, err := net.Listen("tcp", config.ListenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}
	defer listener.Close()

	// Notify the user that the server is listening
	fmt.Printf("Proxy server is listening on %s\n", config.ListenAddr)

	// Set up signal handling for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	defer wg.Wait()

	// Notify the user that the server is ready to accept connections
	fmt.Println("Proxy server is ready to accept connections.")

	// Set a timer for connection timeout
	timeoutTimer := time.NewTimer(time.Second * time.Duration(config.ConnectionTimeout))
	defer timeoutTimer.Stop()

	// Accept incoming connections and handle them
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %s", err)
			continue
		}

		// Choose a backend server randomly
		backend := chooseBackend(config.Server)

		// Increment the WaitGroup counter and notify the user about handling a new connection
		wg.Add(1)
		fmt.Printf("Handling connection from %s. Proxying to backend: %s\n", conn.RemoteAddr(), backend)
		go func() {
			defer wg.Done()
			err := proxy(backend, conn)
			if err != nil {
				log.Printf("failed to proxy: %s", err)
			}
		}()

		// Reset the timer since a new connection is accepted
		timeoutTimer.Reset(time.Second * time.Duration(config.ConnectionTimeout))
	}

	// Wait for the termination signal or timeout
	select {
	case <-sig:
		log.Println("Shutting down...")
		return
	case <-timeoutTimer.C:
		log.Println("Connection timeout reached. Shutting down...")
		return
	}
}

// proxy handles the proxying of data between the client and the backend server
func proxy(backend string, c net.Conn) error {
	defer c.Close()

	// Connect to the chosen backend server with a timeout
	bc, err := net.DialTimeout("tcp", backend, time.Second*5)
	if err != nil {
		return fmt.Errorf("failed to connect to backend %s: %v", backend, err)
	}
	defer bc.Close()

	// Use a WaitGroup to wait for both copy operations to finish
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy data from client to backend
	go func() {
		defer wg.Done()
		_, err := io.Copy(bc, c)
		if err != nil {
			log.Printf("failed to copy from client to backend: %s", err)
		}
	}()

	// Copy data from backend to client
	go func() {
		defer wg.Done()
		_, err := io.Copy(c, bc)
		if err != nil {
			log.Printf("failed to copy from backend to client: %s", err)
		}
	}()

	// Wait for both copy operations to finish
	wg.Wait()

	// Notify the user that the connection has been closed
	fmt.Printf("Connection from %s closed. Proxying to %s terminated.\n", c.RemoteAddr(), backend)

	return nil
}

// chooseBackend selects a backend server randomly
func chooseBackend(servers []string) string {
	rand.Seed(time.Now().UnixNano())
	return servers[rand.Intn(len(servers))]
}

// loadConfig loads configuration from a JSON file or uses defaults
func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		// Use default values if the file is not present
		return &Config{
			ListenAddr:        DefaultListenAddr,
			Server:            append([]string{}, DefaultServers[:]...),
			ConnectionTimeout: DefaultConnectionTimeout,
		}, nil
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &Config{
		ListenAddr:        DefaultListenAddr,
		Server:            append([]string{}, DefaultServers[:]...),
		ConnectionTimeout: DefaultConnectionTimeout,
	}

	err = decoder.Decode(config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode configuration: %v", err)
	}

	return config, nil
}
