# Stdio Pollution Test

This test verifies that the fix for stdio transport pollution is working correctly.

## The Problem

Between versions 1.6.2 and 1.6.4, `fmt.Printf` statements were added to `client/sampling.go` that wrote warning messages directly to stdout. This broke MCP servers using stdio transport because stdout must be exclusively used for JSON-RPC communication.

## The Fix

The `fmt.Printf` statements were replaced with `slog.Default().Warn()` calls that write to stderr instead of stdout.

## Running the Test

```bash
cd examples/stdio_pollution_test
go run main.go
```

If the fix is working correctly, you should see:
```
✅ SUCCESS: stdout is clean!
Warning messages went to logger (stderr) instead of stdout.
This means stdio transport will work correctly for MCP servers.
```

The warnings will still appear on stderr (which is correct), but stdout will remain clean for JSON-RPC communication.

## What This Test Does

1. Captures stdout in a buffer
2. Creates sampling messages with invalid roles (which trigger warnings)
3. Checks if anything was written to stdout
4. Reports success if stdout is clean, failure if polluted 