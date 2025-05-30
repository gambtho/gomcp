package server

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/localrivet/gomcp/mcp"
)

// ProgressNotificationHandler manages progress notifications and rate limiting
type ProgressNotificationHandler struct {
	mu           sync.RWMutex
	rateLimiters map[string]*ProgressRateLimiter // Maps progress tokens to rate limiters
	server       *serverImpl
	channel      *mcp.ProgressChannel // Bidirectional communication channel
	config       *ProgressRateLimitConfig
}

// ProgressRateLimitConfig defines configuration for progress notification rate limiting
type ProgressRateLimitConfig struct {
	// MaxNotificationsPerSecond is the maximum number of notifications allowed per second per token
	MaxNotificationsPerSecond int `json:"maxNotificationsPerSecond"`

	// BufferSize is the maximum number of notifications to buffer when rate limited
	BufferSize int `json:"bufferSize"`

	// OverflowStrategy defines how to handle buffer overflow
	OverflowStrategy OverflowStrategy `json:"overflowStrategy"`

	// CombineThreshold is the minimum time between notifications before combining them
	CombineThreshold time.Duration `json:"combineThreshold"`

	// EnableBatching enables batching of multiple notifications into single messages
	EnableBatching bool `json:"enableBatching"`

	// BatchSize is the maximum number of notifications to batch together
	BatchSize int `json:"batchSize"`

	// BatchTimeout is the maximum time to wait before sending a partial batch
	BatchTimeout time.Duration `json:"batchTimeout"`
}

// OverflowStrategy defines how to handle buffer overflow when rate limiting
type OverflowStrategy int

const (
	// DropOldest drops the oldest notifications when buffer is full
	DropOldest OverflowStrategy = iota

	// DropNewest drops the newest notifications when buffer is full
	DropNewest

	// CombineNotifications combines multiple notifications into summary notifications
	CombineNotifications

	// BlockUntilSpace blocks until buffer space is available (may cause delays)
	BlockUntilSpace
)

// ProgressRateLimiter implements rate limiting for progress notifications with buffering and queue management
type ProgressRateLimiter struct {
	mu                        sync.RWMutex
	maxNotificationsPerSecond int
	lastNotificationTime      time.Time
	notificationCount         int
	windowStart               time.Time

	// Buffering and queue management
	buffer           *list.List // Queue of buffered notifications
	bufferSize       int
	overflowStrategy OverflowStrategy
	combineThreshold time.Duration

	// Batching support
	enableBatching bool
	batchSize      int
	batchTimeout   time.Duration
	currentBatch   []*mcp.ProgressNotification
	batchTimer     *time.Timer

	// Statistics
	totalNotifications    int64
	droppedNotifications  int64
	combinedNotifications int64
	batchedNotifications  int64
}

// NewProgressNotificationHandler creates a new progress notification handler
func NewProgressNotificationHandler(server *serverImpl) *ProgressNotificationHandler {
	config := NewDefaultProgressRateLimitConfig()
	return &ProgressNotificationHandler{
		rateLimiters: make(map[string]*ProgressRateLimiter),
		server:       server,
		channel:      mcp.NewProgressChannel(),
		config:       config,
	}
}

// NewDefaultProgressRateLimitConfig creates a default rate limit configuration
func NewDefaultProgressRateLimitConfig() *ProgressRateLimitConfig {
	return &ProgressRateLimitConfig{
		MaxNotificationsPerSecond: 10,
		BufferSize:                100,
		OverflowStrategy:          CombineNotifications,
		CombineThreshold:          100 * time.Millisecond,
		EnableBatching:            false,
		BatchSize:                 5,
		BatchTimeout:              500 * time.Millisecond,
	}
}

// NewProgressRateLimiter creates a new rate limiter with the given configuration
func NewProgressRateLimiter(config *ProgressRateLimitConfig) *ProgressRateLimiter {
	return &ProgressRateLimiter{
		maxNotificationsPerSecond: config.MaxNotificationsPerSecond,
		windowStart:               time.Now(),
		buffer:                    list.New(),
		bufferSize:                config.BufferSize,
		overflowStrategy:          config.OverflowStrategy,
		combineThreshold:          config.CombineThreshold,
		enableBatching:            config.EnableBatching,
		batchSize:                 config.BatchSize,
		batchTimeout:              config.BatchTimeout,
		currentBatch:              make([]*mcp.ProgressNotification, 0, config.BatchSize),
	}
}

// CanSendNotification checks if a progress notification can be sent based on rate limits
func (prl *ProgressRateLimiter) CanSendNotification() bool {
	prl.mu.Lock()
	defer prl.mu.Unlock()

	now := time.Now()

	// Reset the window if a second has passed
	if now.Sub(prl.windowStart) >= time.Second {
		prl.windowStart = now
		prl.notificationCount = 0
	}

	// Check if we're within the rate limit
	if prl.notificationCount >= prl.maxNotificationsPerSecond {
		return false
	}

	prl.notificationCount++
	prl.lastNotificationTime = now
	return true
}

// BufferNotification adds a notification to the buffer when rate limited
func (prl *ProgressRateLimiter) BufferNotification(notification *mcp.ProgressNotification) error {
	prl.mu.Lock()
	defer prl.mu.Unlock()

	prl.totalNotifications++

	// Handle buffer overflow
	if prl.buffer.Len() >= prl.bufferSize {
		switch prl.overflowStrategy {
		case DropOldest:
			if prl.buffer.Len() > 0 {
				prl.buffer.Remove(prl.buffer.Front())
				prl.droppedNotifications++
			}
		case DropNewest:
			prl.droppedNotifications++
			return fmt.Errorf("buffer full, dropping newest notification")
		case CombineNotifications:
			return prl.combineNotifications(notification)
		case BlockUntilSpace:
			// This would require a more complex implementation with channels
			// For now, we'll treat it as DropOldest
			if prl.buffer.Len() > 0 {
				prl.buffer.Remove(prl.buffer.Front())
				prl.droppedNotifications++
			}
		}
	}

	prl.buffer.PushBack(notification)
	return nil
}

// combineNotifications combines multiple notifications to save buffer space
func (prl *ProgressRateLimiter) combineNotifications(newNotification *mcp.ProgressNotification) error {
	// Find the most recent notification with the same token
	for e := prl.buffer.Back(); e != nil; e = e.Prev() {
		if existing, ok := e.Value.(*mcp.ProgressNotification); ok {
			if existing.Params.ProgressToken == newNotification.Params.ProgressToken {
				// Combine the notifications by updating the existing one
				existing.Params.Progress = newNotification.Params.Progress
				if newNotification.Params.Total != nil {
					existing.Params.Total = newNotification.Params.Total
				}
				if newNotification.Params.Message != "" {
					existing.Params.Message = newNotification.Params.Message
				}
				prl.combinedNotifications++
				return nil
			}
		}
	}

	// If no existing notification found, drop oldest and add new
	if prl.buffer.Len() > 0 {
		prl.buffer.Remove(prl.buffer.Front())
		prl.droppedNotifications++
	}
	prl.buffer.PushBack(newNotification)
	return nil
}

// ProcessBuffer processes buffered notifications that can now be sent
func (prl *ProgressRateLimiter) ProcessBuffer() []*mcp.ProgressNotification {
	prl.mu.Lock()
	defer prl.mu.Unlock()

	var toSend []*mcp.ProgressNotification

	// Process as many buffered notifications as rate limits allow
	for prl.buffer.Len() > 0 && prl.canSendNotificationUnsafe() {
		front := prl.buffer.Front()
		if notification, ok := front.Value.(*mcp.ProgressNotification); ok {
			toSend = append(toSend, notification)
		}
		prl.buffer.Remove(front)
	}

	return toSend
}

// canSendNotificationUnsafe is the unsafe version of CanSendNotification (caller must hold lock)
func (prl *ProgressRateLimiter) canSendNotificationUnsafe() bool {
	now := time.Now()

	// Reset the window if a second has passed
	if now.Sub(prl.windowStart) >= time.Second {
		prl.windowStart = now
		prl.notificationCount = 0
	}

	// Check if we're within the rate limit
	if prl.notificationCount >= prl.maxNotificationsPerSecond {
		return false
	}

	prl.notificationCount++
	prl.lastNotificationTime = now
	return true
}

// GetStatistics returns rate limiting statistics
func (prl *ProgressRateLimiter) GetStatistics() map[string]interface{} {
	prl.mu.RLock()
	defer prl.mu.RUnlock()

	return map[string]interface{}{
		"totalNotifications":    prl.totalNotifications,
		"droppedNotifications":  prl.droppedNotifications,
		"combinedNotifications": prl.combinedNotifications,
		"batchedNotifications":  prl.batchedNotifications,
		"bufferSize":            prl.buffer.Len(),
		"maxBufferSize":         prl.bufferSize,
		"currentRate":           prl.notificationCount,
		"maxRate":               prl.maxNotificationsPerSecond,
	}
}

// SetConfiguration updates the rate limiter configuration
func (pnh *ProgressNotificationHandler) SetConfiguration(config *ProgressRateLimitConfig) {
	pnh.mu.Lock()
	defer pnh.mu.Unlock()
	pnh.config = config

	// Update existing rate limiters with new configuration
	for _, limiter := range pnh.rateLimiters {
		limiter.mu.Lock()
		limiter.maxNotificationsPerSecond = config.MaxNotificationsPerSecond
		limiter.bufferSize = config.BufferSize
		limiter.overflowStrategy = config.OverflowStrategy
		limiter.combineThreshold = config.CombineThreshold
		limiter.enableBatching = config.EnableBatching
		limiter.batchSize = config.BatchSize
		limiter.batchTimeout = config.BatchTimeout
		limiter.mu.Unlock()
	}
}

// GetConfiguration returns the current rate limit configuration
func (pnh *ProgressNotificationHandler) GetConfiguration() *ProgressRateLimitConfig {
	pnh.mu.RLock()
	defer pnh.mu.RUnlock()

	// Return a copy to prevent external modification
	configCopy := *pnh.config
	return &configCopy
}

// GetOrCreateRateLimiter gets or creates a rate limiter for the given progress token
func (pnh *ProgressNotificationHandler) GetOrCreateRateLimiter(progressToken string) *ProgressRateLimiter {
	pnh.mu.Lock()
	defer pnh.mu.Unlock()

	if limiter, exists := pnh.rateLimiters[progressToken]; exists {
		return limiter
	}

	limiter := NewProgressRateLimiter(pnh.config)
	pnh.rateLimiters[progressToken] = limiter
	return limiter
}

// CleanupRateLimiters removes rate limiters for inactive tokens
func (pnh *ProgressNotificationHandler) CleanupRateLimiters() {
	pnh.mu.Lock()
	defer pnh.mu.Unlock()

	for token, limiter := range pnh.rateLimiters {
		// Check if token is still active
		if pnh.server != nil && pnh.server.progressTokenManager != nil {
			if !pnh.server.progressTokenManager.ValidateToken(token) {
				// Token is inactive, clean up the rate limiter
				delete(pnh.rateLimiters, token)

				// Clear any remaining buffer
				limiter.mu.Lock()
				limiter.buffer.Init()
				limiter.mu.Unlock()
			}
		}
	}
}

// GetAllStatistics returns statistics for all rate limiters
func (pnh *ProgressNotificationHandler) GetAllStatistics() map[string]interface{} {
	pnh.mu.RLock()
	defer pnh.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["totalRateLimiters"] = len(pnh.rateLimiters)
	stats["configuration"] = pnh.config

	tokenStats := make(map[string]interface{})
	for token, limiter := range pnh.rateLimiters {
		tokenStats[token] = limiter.GetStatistics()
	}
	stats["tokenStatistics"] = tokenStats

	return stats
}

// GetProgressChannel returns the bidirectional progress communication channel
func (pnh *ProgressNotificationHandler) GetProgressChannel() *mcp.ProgressChannel {
	pnh.mu.RLock()
	defer pnh.mu.RUnlock()
	return pnh.channel
}

// HandleProgressNotification processes a notifications/progress notification
func (s *serverImpl) HandleProgressNotification(message []byte) error {
	// Parse the notification using the new ProgressNotification type
	var notification mcp.ProgressNotification
	if err := notification.FromJSON(message); err != nil {
		return fmt.Errorf("failed to parse progress notification: %w", err)
	}

	// Validate the notification
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid progress notification: %w", err)
	}

	// Validate that the progress token exists and is active
	progressToken := notification.Params.ProgressToken
	if !s.progressTokenManager.ValidateToken(progressToken) {
		s.logger.Debug("received progress notification for unknown or inactive token",
			"progressToken", progressToken)
		return nil // Don't error on unknown tokens, just ignore
	}

	// Update the token's last update time
	if err := s.progressTokenManager.UpdateToken(progressToken); err != nil {
		s.logger.Warn("failed to update progress token", "error", err, "progressToken", progressToken)
	}

	// Log the progress update
	s.logger.Debug("progress notification received",
		"progressToken", progressToken,
		"progress", notification.Params.Progress,
		"total", notification.Params.Total,
		"message", notification.Params.Message,
		"isComplete", notification.IsComplete())

	// Publish to the progress channel for any listeners
	if s.progressNotificationHandler != nil {
		channel := s.progressNotificationHandler.GetProgressChannel()
		if channel != nil && channel.IsActive() {
			if err := channel.Publish(&notification); err != nil {
				s.logger.Warn("failed to publish progress notification to channel", "error", err)
			}
		}
	}

	return nil
}

// SendProgressNotification sends a notifications/progress notification using rate limiting
func (s *serverImpl) SendProgressNotification(progressToken string, progress float64, total *float64, message string) error {
	// Validate that the progress token exists and is active
	if !s.progressTokenManager.ValidateToken(progressToken) {
		return fmt.Errorf("invalid or inactive progress token: %s", progressToken)
	}

	// Get the protocol version for this server
	protocolVersion := s.protocolVersion
	if protocolVersion == "" {
		protocolVersion = "draft" // Default fallback
	}

	// Create the notification using the new type with protocol version awareness
	notification := mcp.NewProgressNotificationForVersion(progressToken, progress, total, message, protocolVersion)

	// Validate the notification
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid progress notification: %w", err)
	}

	return s.sendProgressNotificationWithRateLimit(notification)
}

// SendProgressNotificationDirect sends a progress notification directly using rate limiting
func (s *serverImpl) SendProgressNotificationDirect(notification *mcp.ProgressNotification) error {
	// Set protocol version if not already set
	if notification.GetProtocolVersion() == "" {
		protocolVersion := s.protocolVersion
		if protocolVersion == "" {
			protocolVersion = "draft"
		}
		notification.SetProtocolVersion(protocolVersion)
	}

	// Validate the notification
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid progress notification: %w", err)
	}

	// Validate that the progress token exists and is active
	if !s.progressTokenManager.ValidateToken(notification.Params.ProgressToken) {
		return fmt.Errorf("invalid or inactive progress token: %s", notification.Params.ProgressToken)
	}

	return s.sendProgressNotificationWithRateLimit(notification)
}

// sendProgressNotificationWithRateLimit handles the actual sending with rate limiting
func (s *serverImpl) sendProgressNotificationWithRateLimit(notification *mcp.ProgressNotification) error {
	progressToken := notification.Params.ProgressToken

	// Get or create rate limiter for this token
	var rateLimiter *ProgressRateLimiter
	if s.progressNotificationHandler != nil {
		rateLimiter = s.progressNotificationHandler.GetOrCreateRateLimiter(progressToken)
	}

	// If no rate limiter available, send directly (fallback behavior)
	if rateLimiter == nil {
		return s.sendProgressNotificationDirect(notification)
	}

	// Check if we can send immediately
	if rateLimiter.CanSendNotification() {
		// Send immediately
		if err := s.sendProgressNotificationDirect(notification); err != nil {
			return err
		}

		// Process any buffered notifications that can now be sent
		bufferedNotifications := rateLimiter.ProcessBuffer()
		for _, bufferedNotification := range bufferedNotifications {
			if err := s.sendProgressNotificationDirect(bufferedNotification); err != nil {
				s.logger.Warn("failed to send buffered progress notification",
					"error", err, "progressToken", bufferedNotification.Params.ProgressToken)
			}
		}

		return nil
	}

	// Rate limited - buffer the notification
	if err := rateLimiter.BufferNotification(notification); err != nil {
		s.logger.Warn("failed to buffer progress notification",
			"error", err, "progressToken", progressToken)
		// If buffering fails, try to send directly as fallback
		return s.sendProgressNotificationDirect(notification)
	}

	s.logger.Debug("progress notification buffered due to rate limiting",
		"progressToken", progressToken,
		"bufferSize", rateLimiter.GetStatistics()["bufferSize"])

	return nil
}

// sendProgressNotificationDirect sends a notification without rate limiting (internal method)
func (s *serverImpl) sendProgressNotificationDirect(notification *mcp.ProgressNotification) error {
	// Validate progress increase requirement
	if s.progressTokenManager != nil {
		if lastProgress, err := s.progressTokenManager.GetLastProgress(notification.Params.ProgressToken); err == nil {
			if err := notification.ValidateProgressIncrease(lastProgress); err != nil {
				return fmt.Errorf("progress validation failed: %w", err)
			}
		}
	}

	// Convert to JSON
	messageBytes, err := notification.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal progress notification: %w", err)
	}

	// Send the notification via transport
	if s.transport != nil {
		if err := s.transport.Send(messageBytes); err != nil {
			return fmt.Errorf("failed to send progress notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, progress notification not sent",
			"progressToken", notification.Params.ProgressToken)
	}

	// Publish to the progress channel for any listeners
	if s.progressNotificationHandler != nil {
		channel := s.progressNotificationHandler.GetProgressChannel()
		if channel != nil && channel.IsActive() {
			if err := channel.Publish(notification); err != nil {
				s.logger.Warn("failed to publish progress notification to channel", "error", err)
			}
		}
	}

	// Update the token's progress and last update time
	if s.progressTokenManager != nil {
		if err := s.progressTokenManager.UpdateTokenWithProgress(notification.Params.ProgressToken, notification.Params.Progress); err != nil {
			s.logger.Warn("failed to update progress token after sending notification",
				"error", err, "progressToken", notification.Params.ProgressToken)
		}
	}

	return nil
}

// ProcessBufferedNotifications processes buffered notifications for all tokens (can be called periodically)
func (s *serverImpl) ProcessBufferedNotifications() {
	if s.progressNotificationHandler == nil {
		return
	}

	s.progressNotificationHandler.mu.RLock()
	rateLimiters := make(map[string]*ProgressRateLimiter)
	for token, limiter := range s.progressNotificationHandler.rateLimiters {
		rateLimiters[token] = limiter
	}
	s.progressNotificationHandler.mu.RUnlock()

	for token, limiter := range rateLimiters {
		bufferedNotifications := limiter.ProcessBuffer()
		for _, notification := range bufferedNotifications {
			if err := s.sendProgressNotificationDirect(notification); err != nil {
				s.logger.Warn("failed to send buffered progress notification",
					"error", err, "progressToken", token)
			}
		}
	}
}

// GetProgressRateLimitStatistics returns rate limiting statistics for all tokens
func (s *serverImpl) GetProgressRateLimitStatistics() map[string]interface{} {
	if s.progressNotificationHandler == nil {
		return map[string]interface{}{"error": "progress notification handler not initialized"}
	}

	return s.progressNotificationHandler.GetAllStatistics()
}

// SetProgressRateLimitConfiguration updates the rate limiting configuration
func (s *serverImpl) SetProgressRateLimitConfiguration(config *ProgressRateLimitConfig) {
	if s.progressNotificationHandler != nil {
		s.progressNotificationHandler.SetConfiguration(config)
	}
}

// GetProgressRateLimitConfiguration returns the current rate limiting configuration
func (s *serverImpl) GetProgressRateLimitConfiguration() *ProgressRateLimitConfig {
	if s.progressNotificationHandler == nil {
		return NewDefaultProgressRateLimitConfig()
	}

	return s.progressNotificationHandler.GetConfiguration()
}

// SubscribeToProgress adds a listener for progress notifications with a specific token
func (s *serverImpl) SubscribeToProgress(progressToken string, listener mcp.ProgressListener) {
	if s.progressNotificationHandler != nil {
		channel := s.progressNotificationHandler.GetProgressChannel()
		if channel != nil {
			channel.Subscribe(progressToken, listener)
		}
	}
}

// SubscribeToAllProgress adds a listener for all progress notifications
func (s *serverImpl) SubscribeToAllProgress(listener mcp.ProgressListener) {
	if s.progressNotificationHandler != nil {
		channel := s.progressNotificationHandler.GetProgressChannel()
		if channel != nil {
			channel.SubscribeAll(listener)
		}
	}
}

// UnsubscribeFromProgress removes all listeners for a specific progress token
func (s *serverImpl) UnsubscribeFromProgress(progressToken string) {
	if s.progressNotificationHandler != nil {
		channel := s.progressNotificationHandler.GetProgressChannel()
		if channel != nil {
			channel.Unsubscribe(progressToken)
		}
	}
}

// CreateProgressToken creates a new progress token for a request
func (s *serverImpl) CreateProgressToken(requestID string) string {
	protocolVersion := s.protocolVersion
	if protocolVersion == "" {
		protocolVersion = "draft"
	}
	return s.progressTokenManager.GenerateTokenForVersion(requestID, protocolVersion)
}

// CreateProgressTokenForVersion creates a new progress token for a specific protocol version
func (s *serverImpl) CreateProgressTokenForVersion(requestID string, protocolVersion string) string {
	return s.progressTokenManager.GenerateTokenForVersion(requestID, protocolVersion)
}

// CompleteProgress marks a progress token as completed and sends a final notification
func (s *serverImpl) CompleteProgress(progressToken string, finalMessage string) error {
	// Send a final progress notification (100% complete)
	total := 100.0
	if err := s.SendProgressNotification(progressToken, 100.0, &total, finalMessage); err != nil {
		s.logger.Warn("failed to send final progress notification", "error", err, "progressToken", progressToken)
	}

	// Deactivate the token
	if err := s.progressTokenManager.DeactivateToken(progressToken); err != nil {
		return fmt.Errorf("failed to deactivate progress token: %w", err)
	}

	return nil
}

// CleanupExpiredProgressTokens removes expired progress tokens
// This should be called periodically to prevent memory leaks
func (s *serverImpl) CleanupExpiredProgressTokens(expiration time.Duration) int {
	return s.progressTokenManager.CleanupExpiredTokens(expiration)
}

// CreateProgressReporter creates a new ProgressReporter with the server as the notification sender
func (s *serverImpl) CreateProgressReporter(requestID string, total *float64, initialMessage string) *mcp.ProgressReporter {
	return mcp.NewProgressReporter(mcp.ProgressReporterConfig{
		RequestID:          requestID,
		Total:              total,
		InitialMessage:     initialMessage,
		TokenManager:       s.progressTokenManager,
		NotificationSender: s, // Server implements ProgressNotificationSender
	})
}

// CreateSimpleProgressReporter creates a basic ProgressReporter with minimal configuration
func (s *serverImpl) CreateSimpleProgressReporter(requestID string, total *float64) *mcp.ProgressReporter {
	return mcp.NewProgressReporter(mcp.ProgressReporterConfig{
		RequestID:          requestID,
		Total:              total,
		TokenManager:       s.progressTokenManager,
		NotificationSender: s, // Server implements ProgressNotificationSender
	})
}

// StartProgressOperation is a convenience method that creates a reporter and sends an initial notification
func (s *serverImpl) StartProgressOperation(requestID string, total *float64, initialMessage string) *mcp.ProgressReporter {
	reporter := s.CreateProgressReporter(requestID, total, initialMessage)

	// Send initial notification
	if err := s.SendProgressNotification(reporter.GetToken(), 0.0, total, initialMessage); err != nil {
		s.logger.Warn("failed to send initial progress notification", "error", err, "requestID", requestID)
	}

	return reporter
}

// Note: serverImpl already implements the ProgressNotificationSender interface
// through its existing SendProgressNotification method, so no additional implementation needed
