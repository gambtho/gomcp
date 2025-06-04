package server

import (
	"github.com/localrivet/gomcp/transport/http"
)

// AsHTTP configures the server to use the HTTP transport.
// The HTTP transport allows clients to connect to the server using the standard HTTP protocol,
// sending JSON-RPC requests as HTTP POST requests and receiving responses in the HTTP response body.
//
// Parameters:
//   - address: The listening address for the server (e.g., ":8080" for all interfaces on port 8080)
//   - options: Optional configuration options for the HTTP transport
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsHTTP(address string, options ...http.Option) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create HTTP transport configured for server mode with options
	httpTransport := http.NewServerTransport(address, options...)

	// Configure the transport
	httpTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = httpTransport

	s.logger.Info("server configured with HTTP transport",
		"address", address,
		"api_endpoint", httpTransport.GetFullMCPEndpoint())

	return s
}
