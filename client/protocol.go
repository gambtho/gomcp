// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/localrivet/gomcp/events"
)

// RequestOptions contains options for configuring individual requests
type RequestOptions struct {
	// Timeout specifies the timeout for this specific request
	// If not set, uses the client's default timeout
	Timeout *time.Duration

	// MaxTimeout specifies the maximum timeout regardless of progress notifications
	// If not set, uses 2x the regular timeout as the maximum
	MaxTimeout *time.Duration

	// AllowProgressReset indicates whether progress notifications should reset the timeout clock
	// Default is true as per MCP specification
	AllowProgressReset bool
}

// DefaultRequestOptions returns default request options
func DefaultRequestOptions() *RequestOptions {
	return &RequestOptions{
		AllowProgressReset: true,
	}
}

// WithTimeout sets the timeout for this request
func (opts *RequestOptions) WithTimeout(timeout time.Duration) *RequestOptions {
	opts.Timeout = &timeout
	return opts
}

// WithMaxTimeout sets the maximum timeout for this request
func (opts *RequestOptions) WithMaxTimeout(maxTimeout time.Duration) *RequestOptions {
	opts.MaxTimeout = &maxTimeout
	return opts
}

// WithProgressReset configures whether progress notifications reset the timeout
func (opts *RequestOptions) WithProgressReset(allow bool) *RequestOptions {
	opts.AllowProgressReset = allow
	return opts
}

// progressTracker tracks progress notifications for timeout reset
type progressTracker struct {
	mu                 sync.RWMutex
	requestID          string
	allowProgressReset bool
	lastProgressTime   time.Time
	progressReceived   bool
}

// sendRequest sends a JSON-RPC request to the server and parses the response.
func (c *clientImpl) sendRequest(method string, params interface{}) (interface{}, error) {
	return c.sendRequestWithOptions(method, params, DefaultRequestOptions())
}

// sendRequestWithTimeout sends a JSON-RPC request with a specific timeout.
func (c *clientImpl) sendRequestWithTimeout(method string, params interface{}, timeout time.Duration) (interface{}, error) {
	opts := DefaultRequestOptions().WithTimeout(timeout)
	return c.sendRequestWithOptions(method, params, opts)
}

// sendRequestWithOptions sends a JSON-RPC request with full configuration options.
func (c *clientImpl) sendRequestWithOptions(method string, params interface{}, opts *RequestOptions) (interface{}, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Generate request ID
	requestID := c.generateRequestID()
	requestIDStr := fmt.Sprintf("%d", requestID)

	// Create the request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  method,
	}

	if params != nil {
		request["params"] = params
	}

	// Convert the request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine timeouts
	timeout := c.requestTimeout
	if opts.Timeout != nil {
		timeout = *opts.Timeout
	}

	maxTimeout := timeout * 2 // Default maximum is 2x regular timeout
	if opts.MaxTimeout != nil {
		maxTimeout = *opts.MaxTimeout
	}

	// Create progress tracker if progress reset is enabled
	var tracker *progressTracker
	if opts.AllowProgressReset {
		tracker = &progressTracker{
			requestID:          requestIDStr,
			allowProgressReset: true,
			lastProgressTime:   time.Now(),
		}

		// Register for progress notifications (if supported)
		c.registerProgressTracker(requestIDStr, tracker)
		defer c.unregisterProgressTracker(requestIDStr)
	}

	// Create contexts for timeout management
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	maxCtx, maxCancel := context.WithTimeout(c.ctx, maxTimeout)
	defer maxCancel()

	// Send the request with timeout and progress reset logic
	responseJSON, err := c.sendWithProgressAwareTimeout(ctx, maxCtx, requestJSON, tracker)
	if err != nil {
		// Check if this was a timeout error
		if ctx.Err() == context.DeadlineExceeded || maxCtx.Err() == context.DeadlineExceeded {
			// Send cancellation notification as required by MCP specification
			c.sendCancellationNotification(requestIDStr, "Request timeout")
		}

		// Emit event with actual request JSON and error
		go func() {
			events.Publish[events.RequestFailedEvent](c.events, events.TopicRequestFailed, events.RequestFailedEvent{
				Method:      method,
				RequestJSON: string(requestJSON),
				Error:       err.Error(),
			})
		}()

		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for JSON-RPC errors
	if response.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	// Emit event with actual request and response JSON
	go func() {
		events.Publish[events.ToolExecutedEvent](c.events, events.TopicToolExecuted, events.ToolExecutedEvent{
			Method:       method,
			RequestJSON:  string(requestJSON),
			ResponseJSON: string(responseJSON),
		})
	}()

	return response.Result, nil
}

// sendWithProgressAwareTimeout sends a request with progress-aware timeout reset capability
func (c *clientImpl) sendWithProgressAwareTimeout(ctx, maxCtx context.Context, requestJSON []byte, tracker *progressTracker) ([]byte, error) {
	if tracker == nil || !tracker.allowProgressReset {
		// Simple timeout without progress reset
		return c.transport.SendWithContext(ctx, requestJSON)
	}

	// Progress-aware timeout implementation
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	// Send request in goroutine
	go func() {
		response, err := c.transport.SendWithContext(maxCtx, requestJSON)
		if err != nil {
			errorCh <- err
		} else {
			responseCh <- response
		}
	}()

	// Wait for response with progress-aware timeout
	ticker := time.NewTicker(100 * time.Millisecond) // Check progress every 100ms
	defer ticker.Stop()

	for {
		select {
		case response := <-responseCh:
			return response, nil
		case err := <-errorCh:
			return nil, err
		case <-maxCtx.Done():
			// Maximum timeout exceeded regardless of progress
			return nil, maxCtx.Err()
		case <-ctx.Done():
			// Check if we should reset timeout due to recent progress
			if tracker != nil {
				tracker.mu.RLock()
				progressReceived := tracker.progressReceived
				lastProgress := tracker.lastProgressTime
				tracker.mu.RUnlock()

				if progressReceived && time.Since(lastProgress) < time.Minute {
					// Recent progress received (within last minute), extend timeout
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(c.ctx, c.requestTimeout)
					defer cancel()
					continue
				}
			}
			// No recent progress, timeout
			return nil, ctx.Err()
		case <-ticker.C:
			// Periodic check - continue waiting
			continue
		}
	}
}

// Progress tracking for timeout reset
var (
	progressTrackers = make(map[string]*progressTracker)
	progressMu       sync.RWMutex
)

// registerProgressTracker registers a progress tracker for a request
func (c *clientImpl) registerProgressTracker(requestID string, tracker *progressTracker) {
	progressMu.Lock()
	defer progressMu.Unlock()
	progressTrackers[requestID] = tracker
}

// unregisterProgressTracker removes a progress tracker
func (c *clientImpl) unregisterProgressTracker(requestID string) {
	progressMu.Lock()
	defer progressMu.Unlock()
	delete(progressTrackers, requestID)
}

// handleProgressNotification handles incoming progress notifications for timeout reset
func (c *clientImpl) handleProgressNotification(requestID string) {
	progressMu.RLock()
	tracker, exists := progressTrackers[requestID]
	progressMu.RUnlock()

	if exists && tracker != nil {
		tracker.mu.Lock()
		tracker.progressReceived = true
		tracker.lastProgressTime = time.Now()
		tracker.mu.Unlock()
	}
}

// sendCancellationNotification sends a cancellation notification as required by MCP specification
func (c *clientImpl) sendCancellationNotification(requestID string, reason string) {
	// Create the cancellation notification
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params": map[string]interface{}{
			"requestId": requestID,
		},
	}

	// Add reason if provided
	if reason != "" {
		notification["params"].(map[string]interface{})["reason"] = reason
	}

	// Convert to JSON
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		c.logger.Error("failed to marshal cancellation notification", "error", err, "requestId", requestID)
		return
	}

	// Send the notification (best effort, don't wait for response)
	// Use a short timeout to avoid blocking
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = c.transport.SendWithContext(ctx, notificationJSON)
	if err != nil {
		c.logger.Debug("failed to send cancellation notification", "error", err, "requestId", requestID)
		// Don't return error - this is best effort as per MCP spec
	}
}
