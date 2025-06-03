	// Setup MCP
	mcpClient := setupMCP()
	defer func() {
		// FIXED: Only call registry.StopAll() - it handles client cleanup internally
		// Do NOT call client.Close() before this as it creates a race condition
		if err := mcpClient.registry.StopAll(); err != nil {
			log.Printf("Warning: Error during MCP cleanup: %v", err)
		}
	}() 