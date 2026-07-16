package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("Testing TCP socket connection...")
	fmt.Println()

	// Test 1: Create a TCP server
	fmt.Println("1️⃣ Creating TCP server on 127.0.0.1:9090...")
	listener, err := net.Listen("tcp", "127.0.0.1:9090")
	if err != nil {
		fmt.Printf("❌ Failed to create listener: %v\n", err)
		return
	}
	defer listener.Close()
	fmt.Println("✅ Server listening")

	// Accept connections in background
	connChan := make(chan net.Conn)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("❌ Accept failed: %v\n", err)
			return
		}
		fmt.Println("✅ Client connected!")
		connChan <- conn
	}()

	// Test 2: Wait a bit
	fmt.Println()
	fmt.Println("2️⃣ Waiting 2 seconds...")
	time.Sleep(2 * time.Second)

	// Test 3: Connect as client
	fmt.Println()
	fmt.Println("3️⃣ Connecting as client to 127.0.0.1:9090...")
	client, err := net.Dial("tcp", "127.0.0.1:9090")
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	defer client.Close()
	fmt.Println("✅ Connected as client")

	// Wait for server to accept
	select {
	case <-connChan:
		fmt.Println("✅ Server accepted connection")
	case <-time.After(2 * time.Second):
		fmt.Println("❌ Timeout waiting for accept")
		return
	}

	// Test 4: Send data
	fmt.Println()
	fmt.Println("4️⃣ Sending test message...")
	_, err = client.Write([]byte("Hello, LTT!"))
	if err != nil {
		fmt.Printf("❌ Failed to write: %v\n", err)
		return
	}
	fmt.Println("✅ Message sent")

	fmt.Println()
	fmt.Println("🎉 All tests passed! TCP socket communication works.")
}
