package exporter

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
)

// SocketTransport handles low-level socket communication
type SocketTransport struct {
	path     string
	listener net.Listener
	conn     net.Conn
	mu       sync.RWMutex
	closed   bool
}

// NewSocketTransport creates a new socket transport
func NewSocketTransport(path string) (*SocketTransport, error) {
	st := &SocketTransport{
		path: path,
	}

	if err := st.createSocket(); err != nil {
		return nil, err
	}

	return st, nil
}

// createSocket creates a Unix domain socket (or named pipe on Windows)
func (st *SocketTransport) createSocket() error {
	// Clean up existing socket file
	if runtime.GOOS != "windows" {
		_ = os.Remove(st.path)
	}

	// Create listener based on platform
	var listener net.Listener
	var err error

	if runtime.GOOS == "windows" {
		// Windows: Use named pipes
		listener, err = net.Listen("tcp", st.path)
	} else {
		// Unix: Use domain sockets
		listener, err = net.Listen("unix", st.path)
	}

	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	st.listener = listener

	// Start accepting connections in background
	go st.acceptConnections()

	return nil
}

// acceptConnections accepts incoming connections (typically one TUI viewer)
func (st *SocketTransport) acceptConnections() {
	for {
		conn, err := st.listener.Accept()
		if err != nil {
			st.mu.RLock()
			closed := st.closed
			st.mu.RUnlock()

			if closed {
				return
			}

			time.Sleep(100 * time.Millisecond)
			continue
		}

		st.mu.Lock()
		// Close previous connection if exists
		if st.conn != nil {
			st.conn.Close()
		}
		st.conn = conn
		st.mu.Unlock()
	}
}

// Write writes data to the connected client
// Returns immediately if no client is connected (non-blocking)
func (st *SocketTransport) Write(data []byte) error {
	st.mu.RLock()
	conn := st.conn
	st.mu.RUnlock()

	if conn == nil {
		// No client connected, drop the data silently
		return nil
	}

	// Set write deadline to prevent blocking
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		return err
	}

	// Write with timeout
	_, err := conn.Write(data)
	if err != nil {
		// Connection failed, clear it
		st.mu.Lock()
		if st.conn == conn {
			st.conn.Close()
			st.conn = nil
		}
		st.mu.Unlock()
		return err
	}

	return nil
}

// Read reads data from the socket (used by TUI viewer)
func (st *SocketTransport) Read(conn net.Conn, buf []byte) (int, error) {
	return conn.Read(buf)
}

// Close closes the socket transport
func (st *SocketTransport) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.closed {
		return nil
	}

	st.closed = true

	if st.conn != nil {
		st.conn.Close()
	}

	if st.listener != nil {
		st.listener.Close()
	}

	// Clean up socket file on Unix
	if runtime.GOOS != "windows" {
		_ = os.Remove(st.path)
	}

	return nil
}

// Dial connects to an existing socket (used by TUI viewer)
func Dial(path string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return net.Dial("tcp", path)
	}
	return net.Dial("unix", path)
}

// SocketReader wraps a connection for reading span messages
type SocketReader struct {
	conn   net.Conn
	reader io.Reader
}

// NewSocketReader creates a new socket reader
func NewSocketReader(conn net.Conn) *SocketReader {
	return &SocketReader{
		conn:   conn,
		reader: conn,
	}
}

// ReadMessage reads a single message from the socket
// Returns the raw message bytes (excluding length prefix)
func (sr *SocketReader) ReadMessage(ctx context.Context) ([]byte, error) {
	// Read 4-byte length prefix
	var length uint32
	lengthBuf := make([]byte, 4)

	if _, err := io.ReadFull(sr.reader, lengthBuf); err != nil {
		return nil, err
	}

	length = uint32(lengthBuf[0]) | uint32(lengthBuf[1])<<8 |
	        uint32(lengthBuf[2])<<16 | uint32(lengthBuf[3])<<24

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(sr.reader, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// Close closes the socket reader
func (sr *SocketReader) Close() error {
	return sr.conn.Close()
}
