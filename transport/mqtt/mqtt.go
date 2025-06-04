// Package mqtt provides a MQTT implementation of the MCP transport.
//
// This package implements the Transport interface using MQTT protocol,
// suitable for IoT applications and scenarios where publish/subscribe patterns are useful.
package mqtt

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/localrivet/gomcp/transport"
)

// DefaultQoS is the default Quality of Service level for MQTT
const DefaultQoS = 1 // Default to QoS 1 (at least once)

// DefaultConnectTimeout is the default timeout for connecting to the MQTT broker
const DefaultConnectTimeout = 10 * time.Second

// DefaultTopicPrefix is the default topic prefix for MCP messages
const DefaultTopicPrefix = "mcp"

// DefaultServerTopic is the default topic for server-bound messages
const DefaultServerTopic = "requests"

// DefaultClientTopic is the default topic for client-bound messages
const DefaultClientTopic = "responses"

// Transport implements the transport.Transport interface for MQTT
type Transport struct {
	transport.BaseTransport
	brokerURL    string
	clientID     string
	client       paho.Client
	isServer     bool
	topicPrefix  string
	serverTopic  string
	clientTopic  string
	qos          byte
	username     string
	password     string
	cleanSession bool
	tlsConfig    *TLSConfig
	connected    bool
	subs         map[string]byte
	done         chan struct{}
	handler      transport.MessageHandler
}

// TLSConfig holds TLS configuration for MQTT connections
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	ServerName string
	SkipVerify bool
}

// MQTTOption represents a configuration option for the MQTT transport
type MQTTOption func(*Transport)

// NewTransport creates a new MQTT transport
func NewTransport(brokerURL string, isServer bool, options ...MQTTOption) *Transport {
	t := &Transport{
		brokerURL:    brokerURL,
		isServer:     isServer,
		topicPrefix:  DefaultTopicPrefix,
		serverTopic:  DefaultServerTopic,
		clientTopic:  DefaultClientTopic,
		qos:          DefaultQoS,
		cleanSession: true,
		subs:         make(map[string]byte),
		done:         make(chan struct{}),
	}

	// Generate a random client ID if none is provided
	if t.clientID == "" {
		t.clientID = fmt.Sprintf("mcp-%s-%d", t.roleString(), time.Now().UnixNano())
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// roleString returns a string representing the role (server or client)
func (t *Transport) roleString() string {
	if t.isServer {
		return "server"
	}
	return "client"
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	// Create MQTT client options
	opts := paho.NewClientOptions()
	opts.AddBroker(t.brokerURL)
	opts.SetClientID(t.clientID)
	opts.SetCleanSession(t.cleanSession)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(DefaultConnectTimeout)

	// Set credentials if provided
	if t.username != "" {
		opts.SetUsername(t.username)
		opts.SetPassword(t.password)
	}

	// Configure TLS if provided
	if t.tlsConfig != nil {
		// TLS configuration would be implemented here
		// TODO: Implement TLS configuration for MQTT transport
		slog.Default().Debug("TLS configuration provided but not yet implemented")
	}

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client paho.Client, err error) {
		t.connected = false
		// Could log the connection loss here
	})

	// Set OnConnect handler to resubscribe to topics on reconnection
	opts.SetOnConnectHandler(func(client paho.Client) {
		t.connected = true

		// Resubscribe to topics
		for topic, qos := range t.subs {
			if err := t.subscribe(topic, qos); err != nil {
				// Log error but continue with other subscriptions
				// In a real implementation, you might want to handle this more gracefully
				slog.Default().Error("Failed to resubscribe to topic", "topic", topic, "error", err)
			}
		}
	})

	// Create MQTT client
	t.client = paho.NewClient(opts)

	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if token := t.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.connected = true

	// Server subscribes to request topics with wildcard to catch all client requests
	if t.isServer {
		requestTopic := fmt.Sprintf("%s/%s/+", t.topicPrefix, t.serverTopic)

		if err := t.subscribe(requestTopic, t.qos); err != nil {
			return err
		}
	} else {
		// Client subscribes to its specific response topic
		responseTopic := t.getClientTopic(t.clientID)

		if err := t.subscribe(responseTopic, t.qos); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	close(t.done)

	// Disconnect client
	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(250) // Disconnect with 250ms timeout
	}

	return nil
}

// Send sends a message over the transport
func (t *Transport) Send(message []byte) error {
	if !t.connected {
		return errors.New("not connected to MQTT broker")
	}

	var topic string
	if t.isServer {
		topic = t.getClientTopic("all") // Broadcast to all clients
	} else {
		topic = t.getServerTopic(t.clientID) // Send to server with client ID in topic
	}

	token := t.client.Publish(topic, t.qos, false, message)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Receive is not implemented for MQTT as it uses callbacks
func (t *Transport) Receive() ([]byte, error) {
	return nil, errors.New("not implemented: MQTT transport uses subscription callbacks")
}

// Topic structure:
// - Client sends requests to: {topicPrefix}/requests
// - Server receives requests on: {topicPrefix}/requests
// - Server sends responses to: {topicPrefix}/responses
// - Client receives responses on: {topicPrefix}/responses
// - Server sends requests to: {topicPrefix}/server-requests
// - Client receives server requests on: {topicPrefix}/server-requests
// - Client sends responses to server requests: {topicPrefix}/client-responses
// - Server receives client responses on: {topicPrefix}/client-responses

// messageHandler processes incoming MQTT messages
func (t *Transport) messageHandler(client paho.Client, msg paho.Message) {

	if handler := t.handler; handler != nil {
		response, err := handler(msg.Payload())
		if err != nil {
			slog.Error("message handler error", "error", err)
		} else if response != nil && t.isServer {
			// Extract client ID from the topic to route response securely
			clientID := extractClientIDFromTopic(msg.Topic(), t.topicPrefix, t.serverTopic)

			if clientID != "" {
				// Send response to client-specific topic using client ID
				responseTopic := t.getClientTopic(clientID)

				token := t.client.Publish(responseTopic, t.qos, false, response)
				token.Wait()
			} else {
				// Fallback to broadcast if no client ID found
				responseTopic := t.getClientTopic("all")

				token := t.client.Publish(responseTopic, t.qos, false, response)
				token.Wait()
			}
		}
	}
}

// subscribe subscribes to an MQTT topic
func (t *Transport) subscribe(topic string, qos byte) error {
	token := t.client.Subscribe(topic, qos, t.messageHandler)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	t.subs[topic] = qos

	return nil
}

// MQTT Transport Options

// WithClientID sets the client ID
func WithClientID(clientID string) MQTTOption {
	return func(t *Transport) {
		t.clientID = clientID
	}
}

// WithQoS sets the MQTT Quality of Service level
// QoS 0: At most once delivery
// QoS 1: At least once delivery
// QoS 2: Exactly once delivery
func WithQoS(qos byte) MQTTOption {
	return func(t *Transport) {
		if qos <= 2 {
			t.qos = qos
		}
	}
}

// WithCredentials sets the username and password for MQTT broker authentication
func WithCredentials(username, password string) MQTTOption {
	return func(t *Transport) {
		t.username = username
		t.password = password
	}
}

// WithTopicPrefix sets the topic prefix for MQTT messages
func WithTopicPrefix(prefix string) MQTTOption {
	return func(t *Transport) {
		t.topicPrefix = prefix
	}
}

// WithServerTopic sets the topic name for server-bound messages
func WithServerTopic(topic string) MQTTOption {
	return func(t *Transport) {
		t.serverTopic = topic
	}
}

// WithClientTopic sets the topic name for client-bound messages
func WithClientTopic(topic string) MQTTOption {
	return func(t *Transport) {
		t.clientTopic = topic
	}
}

// WithCleanSession sets whether to start a clean session
func WithCleanSession(clean bool) MQTTOption {
	return func(t *Transport) {
		t.cleanSession = clean
	}
}

// WithTLS sets TLS configuration for secure MQTT connections
func WithTLS(config TLSConfig) MQTTOption {
	return func(t *Transport) {
		t.tlsConfig = &config
	}
}

// SetMessageHandler sets the handler for incoming messages
func (t *Transport) SetMessageHandler(handler transport.MessageHandler) {
	t.handler = handler
}

// HandleMessage processes an incoming message using the registered handler
func (t *Transport) HandleMessage(message []byte) ([]byte, error) {
	handler := t.handler
	if handler == nil {
		return nil, errors.New("no message handler set")
	}

	return handler(message)
}

// getServerTopic returns the full topic for sending to server
func (t *Transport) getServerTopic(clientID string) string {
	if clientID == "" {
		return fmt.Sprintf("%s/%s", t.topicPrefix, t.serverTopic)
	}
	return fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.serverTopic, clientID)
}

// getClientTopic returns the full topic for sending to clients
func (t *Transport) getClientTopic(clientID string) string {
	if clientID == "all" {
		// For publishing responses, use a general response topic
		return fmt.Sprintf("%s/%s", t.topicPrefix, t.clientTopic)
	}
	return fmt.Sprintf("%s/%s/%s", t.topicPrefix, t.clientTopic, clientID)
}

// extractClientIDFromTopic extracts client ID from MQTT topic
func extractClientIDFromTopic(topic, topicPrefix, serverTopic string) string {
	// Expected format: {topicPrefix}/{serverTopic}/{clientID}
	expectedPrefix := fmt.Sprintf("%s/%s/", topicPrefix, serverTopic)
	if len(topic) > len(expectedPrefix) && topic[:len(expectedPrefix)] == expectedPrefix {
		return topic[len(expectedPrefix):]
	}
	return ""
}
