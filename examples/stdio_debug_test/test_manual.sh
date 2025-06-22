#!/bin/bash

echo "🔍 Starting manual stdio MCP debug test..."
echo "=========================================="

# Build the server
echo "📦 Building debug server..."
go build -o debug_server server.go

echo "🚀 Starting debug server and testing manually..."

# Test 1: Just run the server and see what happens
echo ""
echo "Test 1: Starting server to see initial output..."
timeout 3s ./debug_server > test1_output.txt 2> test1_stderr.txt || true

echo "📄 Server stdout (first 3 seconds):"
if [ -s test1_output.txt ]; then
    cat test1_output.txt
else
    echo "(no output)"
fi

echo ""
echo "📄 Server stderr (first 3 seconds):"
if [ -s test1_stderr.txt ]; then
    cat test1_stderr.txt
else
    echo "(no stderr)"
fi

# Test 2: Send initialize and see response
echo ""
echo "Test 2: Sending initialize request..."
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}' > init_request.json

echo "📤 Sending:"
cat init_request.json
echo ""

# Send the request and capture response
timeout 5s ./debug_server < init_request.json > test2_output.txt 2> test2_stderr.txt || true

echo "📥 Server response:"
if [ -s test2_output.txt ]; then
    cat test2_output.txt
else
    echo "(no response)"
fi

echo ""
echo "📄 Server stderr during init:"
if [ -s test2_stderr.txt ]; then
    cat test2_stderr.txt
else
    echo "(no stderr)"
fi

# Check log file
echo ""
echo "📋 Server application logs:"
if [ -s debug_server.log ]; then
    cat debug_server.log
else
    echo "(no log file created)"
fi

echo ""
echo "🔍 Test completed!"
echo "This shows us exactly what the server outputs and any errors." 