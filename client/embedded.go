package client

import (
	"time"

	"github.com/localrivet/gomcp/transport/embedded"
)

// EmbeddedOption is a function that configures an embedded transport.
// These options allow customizing the behavior of the embedded client connection.
type EmbeddedOption func(*embeddedConfig)

// embeddedConfig holds configuration for embedded transport.
// These settings control the behavior of the embedded client connection.
type embeddedConfig struct {
	transport *embedded.Transport
	timeout   time.Duration
}

// WithEmbeddedTimeout sets the timeout for embedded transport operations.
func WithEmbeddedTimeout(timeout time.Duration) EmbeddedOption {
	return func(cfg *embeddedConfig) {
		cfg.timeout = timeout
	}
}

// WithEmbedded configures the client to use embedded (in-process) transport for communication.
//
// Embedded transport provides zero-overhead in-process communication, perfect for
// testing, library integration, and embedded use cases where network overhead
// should be minimized.
//
// The transport parameter should be the client-side transport from a transport pair
// created using embedded.NewTransportPair().
//
// Parameters:
// - transport: The client-side embedded transport from a transport pair
// - options: Optional configuration settings (timeout, buffer size, etc.)
//
// Example:
//
//	// Create transport pair
//	clientTransport, serverTransport := embedded.NewTransportPair()
//
//	// Configure client with the client-side transport
//	client, err := client.NewClient("embedded://",
//	    client.WithEmbedded(clientTransport),
//	    // or with options:
//	    client.WithEmbedded(clientTransport,
//	        client.WithEmbeddedTimeout(5*time.Second)))
func WithEmbedded(transport *embedded.Transport, options ...EmbeddedOption) Option {
	return func(c *clientImpl) {
		// Create default config
		cfg := &embeddedConfig{
			transport: transport,
			timeout:   30 * time.Second,
		}

		// Apply options
		for _, option := range options {
			option(cfg)
		}

		// Create the embedded transport adapter
		embeddedTransport := NewEmbeddedTransport(transport)

		// Set the transport
		c.transport = embeddedTransport

		// Configure timeouts if specified
		if cfg.timeout > 0 {
			c.requestTimeout = cfg.timeout
			c.connectionTimeout = cfg.timeout
			embeddedTransport.SetRequestTimeout(cfg.timeout)
			embeddedTransport.SetConnectionTimeout(cfg.timeout)
		}
	}
}
