#!/bin/bash

echo "🔍 Starting manual stdio MCP debug test..."
echo "=========================================="

# Build the server
echo "📦 Building debug server..."
go build -o debug_server server.go

# Start the server in the background and capture its output
echo "🚀 Starting debug server..."
./debug_server > server_output.txt 2> server_stderr.txt &
SERVER_PID=$!

# Give the server a moment to start
sleep 1

echo "📤 Sending initialize request..."

# Send initialize request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}' | ./debug_server > init_response.txt 2> init_stderr.txt &
INIT_PID=$!

# Wait for response or timeout
sleep 2

echo "📥 Checking initialize response..."
if [ -s init_response.txt ]; then
    echo "✅ Got initialize response:"
    cat init_response.txt
    echo ""
else
    echo "❌ No initialize response received"
fi

echo "📋 Checking server logs..."
if [ -s debug_server.log ]; then
    echo "📄 Server log contents:"
    cat debug_server.log
    echo ""
fi

if [ -s server_stderr.txt ]; then
    echo "⚠️  Server stderr:"
    cat server_stderr.txt
    echo ""
fi

echo "🧹 Cleaning up..."
kill $SERVER_PID 2>/dev/null
kill $INIT_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
wait $INIT_PID 2>/dev/null

echo "🔍 Test completed. Check the output files for details."
echo "Files created:"
echo "  - server_output.txt (server stdout)"
echo "  - server_stderr.txt (server stderr)"  
echo "  - init_response.txt (initialize response)"
echo "  - init_stderr.txt (init client stderr)"
echo "  - debug_server.log (server application logs)" 