package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

var (
	// TODO: configurable
	listenAddr = "localhost:8080"

	// TODO: configurable
	server = []string{
		"localhost:5001",
		"localhost:5002",
		"localhost:5003",
	}
)

func main() {
	listener, err := net.Listen("tcp", listenAddr)

	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %s", err)
		}

		backend := chooseBackend()

		go func() {
			err := proxy(backend, conn)
			if err != nil {
				log.Printf("failed to proxy: %s", err)
			}
		}()
	}
}

func proxy(backend string, c net.Conn) error {
	bc, err := net.Dial("tcp", backend)
	if err != nil {
		return fmt.Errorf("failed to connect to backend %s: %v", backend, err)
	}

	// c -> bc
	go io.Copy(bc, c)

	// bc -> c
	go io.Copy(c, bc)

	return nil

}

func chooseBackend() string {
	// TODO: choose randomly
	return server[0]
}
