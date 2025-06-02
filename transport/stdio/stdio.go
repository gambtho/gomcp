// Package stdio provides a standard I/O implementation of the MCP transport.
//
// This package implements the Transport interface using standard input and output,
// suitable for CLI applications and direct LLM integration.
package stdio

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// isValidJSONRPC checks if a message appears to be a valid JSON-RPC message.
// This provides anti-fragile behavior by filtering out log messages and other noise.
func isValidJSONRPC(data []byte) bool {
	// Quick check: must be valid JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}

	// Must have jsonrpc field with value "2.0"
	jsonrpc, ok := raw["jsonrpc"]
	if !ok {
		return false
	}
	if jsonrpcStr, ok := jsonrpc.(string); !ok || jsonrpcStr != "2.0" {
		return false
	}

	// Must be one of: request (has method + id), response (has id + result/error), or notification (has method, no id)
	_, hasMethod := raw["method"]
	_, hasID := raw["id"]
	_, hasResult := raw["result"]
	_, hasError := raw["error"]

	// Request: method + id
	if hasMethod && hasID {
		return true
	}

	// Response: id + (result or error)
	if hasID && (hasResult || hasError) {
		return true
	}

	// Notification: method, no id
	if hasMethod && !hasID {
		return true
	}

	return false
}

// Transport implements the transport.Transport interface for Standard I/O.
type Transport struct {
	transport.BaseTransport
	reader  *bufio.Reader
	writer  *bufio.Writer
	done    chan struct{}
	readEOF bool
	newline bool // Whether to append a newline to each message
}

// NewTransport creates a new Standard I/O transport.
// By default, it uses os.Stdin and os.Stdout.
func NewTransport() *Transport {
	return NewTransportWithIO(os.Stdin, os.Stdout)
}

// NewTransportWithIO creates a new Standard I/O transport with custom io.Reader and io.Writer.
// This is particularly useful for testing or custom I/O streams.
func NewTransportWithIO(in io.Reader, out io.Writer) *Transport {
	return &Transport{
		reader:  bufio.NewReader(in),
		writer:  bufio.NewWriter(out),
		done:    make(chan struct{}),
		newline: true, // Default to appending newlines
	}
}

// Initialize initializes the transport.
func (t *Transport) Initialize() error {
	// Nothing to initialize for stdio transport
	return nil
}

// Start starts the transport, beginning to read from stdin.
func (t *Transport) Start() error {
	// Start a goroutine to read from stdin
	go t.readLoop()
	return nil
}

// Stop stops the transport, closing the done channel.
func (t *Transport) Stop() error {
	// Signal the read loop to stop
	close(t.done)
	return nil
}

// Send sends a message over stdout.
func (t *Transport) Send(message []byte) error {
	// Write the message to stdout
	_, err := t.writer.Write(message)
	if err != nil {
		return err
	}

	// Add newline if configured
	if t.newline {
		_, err = t.writer.WriteString("\n")
		if err != nil {
			return err
		}
	}

	return t.writer.Flush()
}

// Receive is not implemented for stdio transport as it uses the readLoop.
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("not implemented: stdio transport uses readLoop with handler")
}

// SetNewline configures whether to append a newline to each sent message.
func (t *Transport) SetNewline(newline bool) {
	t.newline = newline
}

// readLoop reads messages from stdin and passes them to the handler.
func (t *Transport) readLoop() {
	for {
		select {
		case <-t.done:
			return
		default:
			// Read a line from stdin
			line, err := t.reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// EOF doesn't mean we should exit - the parent process might send more input later
					// Just sleep a bit to avoid tight loop
					t.readEOF = true

					// Log EOF for debugging
					if debugHandler := t.GetDebugHandler(); debugHandler != nil {
						debugHandler("stdio transport: received EOF, waiting for more input")
					}

					// Sleep briefly to avoid CPU spin
					select {
					case <-t.done:
						return
					default:
						// Sleep for a short time then check again
						time.Sleep(100 * time.Millisecond)
						continue
					}
				}

				// For other errors, log and continue
				if debugHandler := t.GetDebugHandler(); debugHandler != nil {
					debugHandler("stdio transport error: " + err.Error())
				}
				continue
			}

			// Reset EOF flag if we got a line
			t.readEOF = false

			// Trim newline character(s)
			line = strings.TrimRight(line, "\r\n")

			// Skip empty lines
			if line == "" {
				continue
			}

			// Anti-fragile filtering: only process valid JSON-RPC messages
			if !isValidJSONRPC([]byte(line)) {
				// Log filtered message if debug enabled
				if debugHandler := t.GetDebugHandler(); debugHandler != nil {
					if len(line) > 100 {
						debugHandler("stdio transport filtered non-JSON-RPC: " + line[:100] + "...")
					} else {
						debugHandler("stdio transport filtered non-JSON-RPC: " + line)
					}
				}
				continue
			}

			// Log received message if debug enabled
			if debugHandler := t.GetDebugHandler(); debugHandler != nil {
				if len(line) > 100 {
					debugHandler("stdio transport received: " + line[:100] + "...")
				} else {
					debugHandler("stdio transport received: " + line)
				}
			}

			// Process the message with the handler
			if response, err := t.HandleMessage([]byte(line)); err == nil && response != nil {
				if err := t.Send(response); err != nil {
					// Log error but continue processing
					if debugHandler := t.GetDebugHandler(); debugHandler != nil {
						debugHandler("stdio transport: failed to send response: " + err.Error())
					}
				}
			}
		}
	}
}
