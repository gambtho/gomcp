package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Configure slog to write to stderr (not stdout) to avoid capturing our logs
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	// Create messages with invalid roles - these should trigger warnings
	_ = client.CreateTextMessage("system", "This should trigger a warning")
	_ = client.CreateImageMessage("admin", "image_data", "image/png")
	_ = client.CreateAudioMessage("moderator", "audio_data", "audio/wav")

	// Close the pipe and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read what was written to stdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Check if stdout is clean (no output should be captured)
	if buf.Len() > 0 {
		println("❌ FAILED: stdout was polluted with:", buf.String())
		println("This would break stdio transport for MCP servers!")
		os.Exit(1)
	} else {
		println("✅ SUCCESS: stdout is clean!")
		println("Warning messages went to logger (stderr) instead of stdout.")
		println("This means stdio transport will work correctly for MCP servers.")
	}
}
