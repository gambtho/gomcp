// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"encoding/json"
	"fmt"
)

// AddRoot adds a filesystem root to be exposed to the server.
// Accepts either filesystem paths or file:// URIs - automatically converts paths to file:// URIs as required by MCP specification.
func (c *clientImpl) AddRoot(uri string, name string) error {
	// Convert to file:// URI if needed (MCP specification requires file:// URIs)
	uri = ensureFileURI(uri)

	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()

	// Check if the root already exists
	for _, root := range c.roots {
		if root.URI == uri {
			return fmt.Errorf("root with URI %s already exists", uri)
		}
	}

	// Add the root to our local cache
	c.roots = append(c.roots, Root{
		URI:  uri,
		Name: name,
	})

	// Note: According to MCP protocol, there is no "roots/add" method.
	// Clients manage their own roots and only notify servers of changes.

	// Enable roots capability if not already enabled
	if !c.capabilities.Roots.ListChanged {
		c.capabilities.Roots.ListChanged = true
	}

	// Send notification that the roots list has changed
	if err := c.sendRootsListChangedNotification(); err != nil {
		// Log error but don't fail the operation
		// The root was successfully added, notification failure shouldn't break that
		// TODO: Add proper logging when available
		_ = err
	}

	return nil
}

// RemoveRoot removes a filesystem root.
// Accepts either filesystem paths or file:// URIs - automatically converts paths to file:// URIs as required by MCP specification.
func (c *clientImpl) RemoveRoot(uri string) error {
	// Convert to file:// URI if needed (MCP specification requires file:// URIs)
	uri = ensureFileURI(uri)

	c.rootsMu.Lock()
	defer c.rootsMu.Unlock()

	// Find the root in our local cache
	var foundIndex = -1
	for i, root := range c.roots {
		if root.URI == uri {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("root with URI %s not found", uri)
	}

	// Note: According to MCP protocol, there is no "roots/remove" method.
	// Clients manage their own roots and only notify servers of changes.

	// Remove the root from our local cache
	c.roots = append(c.roots[:foundIndex], c.roots[foundIndex+1:]...)

	// Send notification that the roots list has changed
	if err := c.sendRootsListChangedNotification(); err != nil {
		// Log error but don't fail the operation
		// The root was successfully removed, notification failure shouldn't break that
		// TODO: Add proper logging when available
		_ = err
	}

	return nil
}

// GetRoots returns the current list of roots.
// According to MCP protocol, clients manage their own roots.
// The server requests roots via "roots/list" calls TO the client, not FROM the client.
func (c *clientImpl) GetRoots() ([]Root, error) {
	c.rootsMu.RLock()
	defer c.rootsMu.RUnlock()

	// Return a copy to prevent modifications
	roots := make([]Root, len(c.roots))
	copy(roots, c.roots)

	return roots, nil
}

// handleRootsList handles a roots/list request from the server.
func (c *clientImpl) handleRootsList(requestID int64) error {
	c.rootsMu.RLock()
	roots := make([]Root, len(c.roots))
	copy(roots, c.roots)
	c.rootsMu.RUnlock()

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result": map[string]interface{}{
			"roots": roots,
		},
	}

	// Convert to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal roots/list response: %w", err)
	}

	// Send the response
	_, err = c.transport.Send(responseJSON)
	if err != nil {
		return fmt.Errorf("failed to send roots/list response: %w", err)
	}

	return nil
}

// sendRootsListChangedNotification sends a notification that the roots list has changed.
func (c *clientImpl) sendRootsListChangedNotification() error {
	// Check if we support the roots/list_changed notification
	if !c.capabilities.Roots.ListChanged {
		return nil
	}

	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/roots/list_changed",
	}

	// Convert to JSON
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal roots list changed notification: %w", err)
	}

	// Send the notification
	_, err = c.transport.Send(notificationJSON)
	if err != nil {
		return fmt.Errorf("failed to send roots list changed notification: %w", err)
	}

	return nil
}

// isValidFileURI validates that the URI is a valid file:// URI according to MCP specification
func isValidFileURI(uri string) bool {
	// According to MCP specification, root URIs MUST be file:// URIs
	return len(uri) > 7 && uri[:7] == "file://"
}

// ensureFileURI converts a filesystem path to a file:// URI as required by MCP specification.
// If the input is already a valid file:// URI, it returns it unchanged.
func ensureFileURI(path string) string {
	if isValidFileURI(path) {
		return path // Already a valid file:// URI
	}

	// Convert backslashes to forward slashes (Windows compatibility)
	normalizedPath := path
	for i, char := range normalizedPath {
		if char == '\\' {
			normalizedPath = normalizedPath[:i] + "/" + normalizedPath[i+1:]
		}
	}

	// Ensure path starts with /
	if len(normalizedPath) == 0 || normalizedPath[0] != '/' {
		normalizedPath = "/" + normalizedPath
	}

	return "file://" + normalizedPath
}
