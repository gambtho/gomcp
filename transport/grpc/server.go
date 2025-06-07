package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc"
)

// mcpServer implements the MCP gRPC server.
// It handles incoming client requests and streams messages
// between clients and the MCP server.
type mcpServer struct {
	pb.UnimplementedMCPServer
	transport *Transport
}

// startGRPCServer starts the gRPC server.
//
// This method creates a TCP listener, registers the MCP service,
// and starts the gRPC server in a background goroutine.
// It uses the transport's configured address and options.
func (t *Transport) startGRPCServer() error {
	// Check if we have a valid address
	if t.address == "" {
		t.address = fmt.Sprintf(":%d", DefaultPort)
	} else if !strings.Contains(t.address, ":") {
		t.address = fmt.Sprintf("%s:%d", t.address, DefaultPort)
	}

	// Create listener
	lis, err := net.Listen("tcp", t.address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create server options
	opts := t.getServerOptions()

	// Create server
	t.server = grpc.NewServer(opts...)

	// Register service
	pb.RegisterMCPServer(t.server, &mcpServer{transport: t})

	// Start server in a goroutine
	go func() {
		if err := t.server.Serve(lis); err != nil {
			select {
			case t.errCh <- fmt.Errorf("failed to serve: %w", err):
			case <-t.ctx.Done():
				// Context is done, server is shutting down
			}
		}
	}()

	return nil
}

// Initialize handles the Initialize RPC.
//
// This RPC is called by clients to establish a session with the server.
// It returns session information and server version details.
func (s *mcpServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	// For MCP, initialization is handled via JSON-RPC, not gRPC native calls
	// Return basic success response
	resp := &pb.InitializeResponse{
		SessionId:     "grpc-session-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		ServerVersion: "1.0.0",
		Success:       true,
	}
	return resp, nil
}

// StreamMessages implements bidirectional streaming for message exchange.
//
// This method acts as a message pipe, passing JSON-RPC messages between
// the client and the MCP server's message handler, just like other transports.
func (s *mcpServer) StreamMessages(stream pb.MCP_StreamMessagesServer) error {
	// Create done channel for this stream
	done := make(chan struct{})
	defer close(done)

	// Start a goroutine to send outgoing messages to the client
	go func() {
		defer func() {
			// Recover from any panic, particularly send on closed channel
			if r := recover(); r != nil {
				s.transport.GetLogger().Warn("Sender goroutine recovered from panic", "error", r)
			}
		}()

		for {
			select {
			case <-s.transport.ctx.Done():
				return
			case message, ok := <-s.transport.sendCh:
				if !ok {
					// Channel closed, exit gracefully
					return
				}

				s.transport.GetLogger().Info("Sending message to client", "content", string(message))

				// Convert the JSON-RPC message to gRPC format
				protoMsg := &pb.MCPMessage{
					Id: "msg-" + fmt.Sprintf("%d", time.Now().UnixNano()),
					Content: &pb.MCPMessage_TextContent{
						TextContent: string(message),
					},
					Timestamp: uint64(time.Now().UnixMilli()),
				}

				// Send the message to the client
				if err := stream.Send(protoMsg); err != nil {
					// Log error but don't crash
					s.transport.GetLogger().Warn("Failed to send message to client", "error", err)
					return
				}

				s.transport.GetLogger().Info("Successfully sent message to client")
			}
		}
	}()

	// Receive messages from the client and put them in recvCh
	// The server will process them via transport.Receive() and send responses via transport.Send()
	for {
		protoMsg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Stream closed by client - normal termination
				return nil
			}
			return fmt.Errorf("failed to receive message: %w", err)
		}

		// Extract the JSON-RPC message content
		var message []byte
		switch content := protoMsg.Content.(type) {
		case *pb.MCPMessage_TextContent:
			message = []byte(content.TextContent)
		case *pb.MCPMessage_BinaryContent:
			message = content.BinaryContent
		default:
			// Skip unknown content types
			s.transport.GetLogger().Warn("Unknown message content type", "type", fmt.Sprintf("%T", content))
			continue
		}

		s.transport.GetLogger().Info("Received message from client", "content", string(message))

		// Process the message directly using the transport's handler (like stdio transport does)
		if response, err := s.transport.HandleMessage(message); err == nil && response != nil {
			s.transport.GetLogger().Info("Generated response", "content", string(response))

			// Send the response back via sendCh
			select {
			case s.transport.sendCh <- response:
				// Response queued for sending
			case <-s.transport.ctx.Done():
				return fmt.Errorf("send canceled: %w", s.transport.ctx.Err())
			default:
				// Channel is full, log warning
				s.transport.GetLogger().Warn("Send channel full, dropping response")
			}
		} else if err != nil {
			s.transport.GetLogger().Warn("Message handler error", "error", err)
		}
	}
}

// StreamEvents implements server-to-client event streaming.
func (s *mcpServer) StreamEvents(req *pb.EventStreamRequest, stream pb.MCP_StreamEventsServer) error {
	// TODO: Implement event streaming if needed for MCP
	return fmt.Errorf("event streaming not implemented")
}

// ExecuteFunction executes a function and returns the result.
func (s *mcpServer) ExecuteFunction(ctx context.Context, req *pb.FunctionRequest) (*pb.FunctionResponse, error) {
	// MCP uses JSON-RPC for function calls, not gRPC native calls
	// Return not implemented to encourage using the streaming interface
	return nil, fmt.Errorf("use StreamMessages for MCP communication")
}

// EndSession terminates an active MCP session.
func (s *mcpServer) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	// MCP session management is handled via JSON-RPC
	return &pb.EndSessionResponse{
		Success: true,
	}, nil
}
