package server

import (
	"github.com/localrivet/gomcp/transport/embedded"
)

// AsEmbedded configures the server to use embedded (in-process) transport for communication.
//
// Embedded transport provides zero-overhead in-process communication, perfect for
// testing, library integration, and embedded use cases where network overhead
// should be minimized.
//
// The transport parameter should be the server-side transport from a transport pair
// created using embedded.NewTransportPair().
//
// Parameters:
//   - transport: The server-side embedded transport instance
//
// Example:
//
//	// Create transport pair
//	serverTransport, clientTransport := embedded.NewTransportPair()
//
//	// Configure server with the server-side transport
//	server.AsEmbedded(serverTransport)
//
//	// Use clientTransport with your MCP client
//	client := client.NewEmbeddedTransport(clientTransport)
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsEmbedded(transport *embedded.Transport) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Configure the message handler
	transport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = transport

	s.logger.Info("server configured with embedded transport")
	return s
}
