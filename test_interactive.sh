#!/bin/bash

echo "🔍 Interactive stdio MCP debug test..."
echo "======================================"

# Build the server
echo "📦 Building debug server..."
go build -o debug_server server.go

echo "🚀 Starting server in background..."

# Create named pipes for bidirectional communication
mkfifo server_input server_output 2>/dev/null || true

# Start server with named pipes
./debug_server < server_input > server_output 2> server_stderr.txt &
SERVER_PID=$!

# Give server time to start
sleep 1

echo "📤 Sending initialize request..."

# Send initialize request (keep pipe open)
{
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}'
    sleep 2
    echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
    sleep 1
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
    sleep 2
} > server_input &
INPUT_PID=$!

echo "📥 Reading server responses..."

# Read responses for a few seconds
timeout 5s cat server_output &
OUTPUT_PID=$!

# Wait for responses
sleep 6

echo ""
echo "🧹 Cleaning up..."
kill $SERVER_PID $INPUT_PID $OUTPUT_PID 2>/dev/null
wait $SERVER_PID $INPUT_PID $OUTPUT_PID 2>/dev/null

echo "📋 Server stderr:"
if [ -s server_stderr.txt ]; then
    cat server_stderr.txt
else
    echo "(no stderr)"
fi

echo ""
echo "📋 Server application logs:"
if [ -s debug_server.log ]; then
    cat debug_server.log
else
    echo "(no log file)"
fi

# Cleanup
rm -f server_input server_output

echo ""
echo "🔍 Interactive test completed!" 