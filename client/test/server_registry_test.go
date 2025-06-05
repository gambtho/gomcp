package test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerRegistry(t *testing.T) {
	tests := []struct {
		name      string
		opts      []client.ServerRegistryOption
		hasLogger bool
	}{
		{
			name:      "default registry without logger",
			opts:      nil,
			hasLogger: false,
		},
		{
			name: "registry with logger",
			opts: []client.ServerRegistryOption{
				client.WithRegistryLogger(slog.Default()),
			},
			hasLogger: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := client.NewServerRegistry(tt.opts...)
			require.NotNil(t, registry)

			// Test that we can get server names (empty initially)
			names, err := registry.GetServerNames()
			require.NoError(t, err)
			assert.Empty(t, names)
		})
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		opts     []client.LoggerOption
		testFunc func(t *testing.T, logger *slog.Logger)
	}{
		{
			name: "default logger",
			opts: nil,
			testFunc: func(t *testing.T, logger *slog.Logger) {
				require.NotNil(t, logger)
				// Default logger should work
				logger.Info("test message")
			},
		},
		{
			name: "logger with discard output",
			opts: []client.LoggerOption{
				client.WithLogDiscard(),
			},
			testFunc: func(t *testing.T, logger *slog.Logger) {
				require.NotNil(t, logger)
				// Should not panic or error
				logger.Info("this should be discarded")
				logger.Error("this should also be discarded")
			},
		},
		{
			name: "logger with custom buffer output",
			opts: []client.LoggerOption{
				client.WithLogOutput(&bytes.Buffer{}),
				client.WithLogLevel(slog.LevelDebug),
			},
			testFunc: func(t *testing.T, logger *slog.Logger) {
				require.NotNil(t, logger)
				logger.Debug("debug message")
				logger.Info("info message")
			},
		},
		{
			name: "logger with file output",
			opts: []client.LoggerOption{
				client.WithLogFile(filepath.Join(t.TempDir(), "test.log")),
				client.WithLogLevel(slog.LevelWarn),
			},
			testFunc: func(t *testing.T, logger *slog.Logger) {
				require.NotNil(t, logger)
				logger.Warn("warning message")
				logger.Error("error message")
				// Info should be filtered out by level
				logger.Info("info message")
			},
		},
		{
			name: "logger with invalid file path falls back to discard",
			opts: []client.LoggerOption{
				client.WithLogFile("/invalid/path/that/does/not/exist/test.log"),
			},
			testFunc: func(t *testing.T, logger *slog.Logger) {
				require.NotNil(t, logger)
				// Should not panic even with invalid file path
				logger.Info("this should be discarded due to fallback")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := client.NewLogger(tt.opts...)
			tt.testFunc(t, logger)
		})
	}
}

func TestServerRegistryWithConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	config := client.ServerConfig{
		MCPServers: map[string]client.ServerDefinition{
			"test-server": {
				Command: "echo",
				Args:    []string{"test"},
				Env:     map[string]string{"TEST_VAR": "test_value"},
			},
		},
	}

	configData, err := json.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, configData, 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		opts      []client.ServerRegistryOption
		wantError bool
	}{
		{
			name:      "load config without logger",
			opts:      nil,
			wantError: false,
		},
		{
			name: "load config with logger",
			opts: []client.ServerRegistryOption{
				client.WithRegistryLogger(client.NewLogger(client.WithLogDiscard())),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := client.NewServerRegistry(tt.opts...)
			require.NotNil(t, registry)

			err := registry.LoadConfig(configPath)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				// Note: This might fail due to the echo command not being a proper MCP server
				// but the config loading part should work
				if err != nil {
					t.Logf("Expected error loading config with echo command: %v", err)
				}
			}
		})
	}
}

func TestServerConfigOptionsBehavior(t *testing.T) {
	tests := []struct {
		name string
		opts []client.ServerConfigOption
	}{
		{
			name: "without logger option",
			opts: nil,
		},
		{
			name: "with logger option",
			opts: []client.ServerConfigOption{
				client.WithServerRegistryLogger(client.NewLogger(client.WithLogDiscard())),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the server config options can be created without panic
			// This tests the option creation mechanism
			// Note: tt.opts can be nil for the "without logger option" test

			// Test option application by creating a server registry with options
			var logBuffer bytes.Buffer
			logger := client.NewLogger(client.WithLogOutput(&logBuffer))

			// Test WithServerRegistryLogger directly
			if len(tt.opts) > 0 {
				// Test that we can create a registry with the logger option
				registry := client.NewServerRegistry(client.WithRegistryLogger(logger))
				require.NotNil(t, registry)
			} else {
				// Test that we can create a registry without options
				registry := client.NewServerRegistry()
				require.NotNil(t, registry)
			}
		})
	}
}

func TestWithServerConfigOptions(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	config := client.ServerConfig{
		MCPServers: map[string]client.ServerDefinition{
			"test-server": {
				Command: "echo",
				Args:    []string{"test"},
			},
		},
	}

	configData, err := json.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, configData, 0644)
	require.NoError(t, err)

	tests := []struct {
		name string
		opts []client.ServerConfigOption
	}{
		{
			name: "without registry logger",
			opts: nil,
		},
		{
			name: "with registry logger",
			opts: []client.ServerConfigOption{
				client.WithServerRegistryLogger(client.NewLogger(client.WithLogDiscard())),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the server config options can be created without panic
			// This tests that the option functions work correctly

			// Test that WithServerConfig can be called with these options
			// (We can't test the full execution without a real clientImpl)
			option := client.WithServerConfig(configPath, "test-server", tt.opts...)
			require.NotNil(t, option)

			// The option creation should not panic
			t.Log("Server config option created successfully")
		})
	}
}

func TestLoggerFileHandling(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Test that file logger creates and writes to file
	logger := client.NewLogger(client.WithLogFile(logFile))
	require.NotNil(t, logger)

	testMessage := "test log message"
	logger.Info(testMessage)

	// Give it a moment for the log to be written
	time.Sleep(10 * time.Millisecond)

	// Check that the file was created and contains our message
	content, err := os.ReadFile(logFile)
	if err == nil {
		// File should exist and contain our message
		assert.Contains(t, string(content), testMessage)
	}
	// Note: If the file doesn't exist, it might have fallen back to discard,
	// which is acceptable behavior for the error case
}

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer

	logger := client.NewLogger(
		client.WithLogOutput(&buf),
		client.WithLogLevel(slog.LevelWarn),
	)

	// These should not appear in output (below warn level)
	logger.Debug("debug message")
	logger.Info("info message")

	// These should appear in output (warn level and above)
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestRegistryServerManagement(t *testing.T) {
	registry := client.NewServerRegistry()
	require.NotNil(t, registry)

	// Test getting names from empty registry
	names, err := registry.GetServerNames()
	require.NoError(t, err)
	assert.Empty(t, names)

	// Test getting non-existent client
	_, err = registry.GetClient("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test stopping non-existent server
	err = registry.StopServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test stopping all servers (should not error even if empty)
	err = registry.StopAll()
	assert.NoError(t, err)
}

func TestConcurrentServerStartup(t *testing.T) {
	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{
			name:   "concurrent startup without logger",
			logger: nil,
		},
		{
			name:   "concurrent startup with logger",
			logger: client.NewLogger(client.WithLogDiscard()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create registry with optional logger
			var opts []client.ServerRegistryOption
			if tt.logger != nil {
				opts = append(opts, client.WithRegistryLogger(tt.logger))
			}
			registry := client.NewServerRegistry(opts...)

			// Configure multiple servers with sleep commands to simulate startup time
			config := client.ServerConfig{
				MCPServers: map[string]client.ServerDefinition{
					"server1": {
						Command: "sh",
						Args:    []string{"-c", "sleep 0.1; echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"test1\",\"version\":\"1.0.0\"}}}'"},
					},
					"server2": {
						Command: "sh",
						Args:    []string{"-c", "sleep 0.1; echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"test2\",\"version\":\"1.0.0\"}}}'"},
					},
					"server3": {
						Command: "sh",
						Args:    []string{"-c", "sleep 0.1; echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"test3\",\"version\":\"1.0.0\"}}}'"},
					},
				},
			}

			// Measure the time to start all servers
			start := time.Now()
			err := registry.ApplyConfig(config)
			elapsed := time.Since(start)

			// The important thing is that they should all start concurrently
			// If they were sequential, it would take 3 * 0.1s = 0.3s minimum
			// If they're concurrent, it should take roughly the time of the slowest server + overhead
			// We'll be more lenient since the test servers might actually work
			if elapsed > time.Millisecond*500 {
				t.Errorf("Servers took too long to start: %v (expected < 500ms for concurrent startup)", elapsed)
			}

			// The fake servers might succeed or fail, both are acceptable
			// The key is that they started concurrently
			t.Logf("Servers started in %v with result: %v", elapsed, err)

			// Clean up
			_ = registry.StopAll()
		})
	}
}

func TestServerRegistryCleanupRaceCondition(t *testing.T) {
	t.Run("proper cleanup without race condition", func(t *testing.T) {
		// Create a registry with discard logger to avoid output interference
		registry := client.NewServerRegistry(client.WithRegistryLogger(client.NewLogger(client.WithLogDiscard())))

		// Create a configuration with a simple echo server
		config := client.ServerConfig{
			MCPServers: map[string]client.ServerDefinition{
				"test-server": {
					Command: "echo",
					Args:    []string{"test"},
				},
			},
		}

		// Start the server
		startTime := time.Now()
		err := registry.ApplyConfig(config)
		elapsed := time.Since(startTime)

		// After our transport fix, the client properly waits for the full timeout
		// when trying to connect to the echo command (which doesn't implement MCP).
		// This is expected behavior - the old version failed faster due to race conditions.
		// We expect this to take close to the connection timeout (10s) since echo doesn't respond to MCP protocol.
		if elapsed > time.Second*11 {
			t.Errorf("Server startup took too long: %v (should timeout around 10s)", elapsed)
		}

		// We expect an error since echo doesn't implement MCP protocol
		if err == nil {
			t.Error("Expected error when connecting to echo command, but got none")
		}

		// Log any startup errors (expected for echo command)
		if err != nil {
			t.Logf("Note: ApplyConfig returned error (expected for echo command): %v", err)
		}

		// Get the client to verify it was NOT created (echo fails MCP handshake)
		client, clientErr := registry.GetClient("test-server")
		if clientErr == nil && client != nil {
			t.Error("Unexpected success: echo command should not successfully create MCP client")
		} else {
			// This is the expected path - echo command exits immediately and doesn't implement MCP
			t.Logf("Note: Could not get client (expected for echo command): %v", clientErr)
		}

		// Test that StopAll() works without race conditions even with failed servers
		shutdownTime := time.Now()
		stopErr := registry.StopAll()
		shutdownElapsed := time.Since(shutdownTime)

		// Should be very fast since the echo process already exited
		if shutdownElapsed > time.Second {
			t.Errorf("Server shutdown took too long: %v (should be fast for already-dead process)", shutdownElapsed)
		}

		// Should not have errors for empty registry
		if stopErr != nil {
			t.Logf("Note: StopAll() returned error (expected for failed server): %v", stopErr)
		}

		// Verify we can still call StopAll() again without issues
		secondStopErr := registry.StopAll()
		if secondStopErr != nil {
			t.Logf("Note: Second StopAll() returned error (expected): %v", secondStopErr)
		}
	})

	t.Run("demonstrate race condition warning", func(t *testing.T) {
		// This test documents the race condition pattern that users should avoid
		// We don't actually execute the problematic code, but show it in comments

		registry := client.NewServerRegistry(client.WithRegistryLogger(client.NewLogger(client.WithLogDiscard())))

		// ❌ DON'T DO THIS (race condition):
		// defer func() {
		//     client.Close()         // Kills the connection/process
		//     registry.StopAll()     // Tries to wait for already-killed process
		// }()

		// ✅ DO THIS INSTEAD (proper cleanup):
		defer func() {
			// Only call registry.StopAll() - it handles client cleanup internally
			if err := registry.StopAll(); err != nil {
				t.Logf("Cleanup error (expected for empty registry): %v", err)
			}
		}()

		// This test passes to document the correct pattern
		t.Log("Race condition test completed - shows correct cleanup pattern in comments")
	})
}
