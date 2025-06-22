#!/bin/bash

echo "🔍 Detailed stdio MCP debug test..."
echo "==================================="

echo "📦 Testing individual requests..."

# Test 1: Just initialize
echo ""
echo "Test 1: Initialize only"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}' | timeout 3s ./debug_server > test1_response.txt 2> test1_stderr.txt

echo "Response:"
cat test1_response.txt 2>/dev/null || echo "(no response)"

# Test 2: Initialize + initialized notification
echo ""
echo "Test 2: Initialize + initialized notification"
{
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}'
    sleep 0.1
    echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
    sleep 1
} | timeout 5s ./debug_server > test2_response.txt 2> test2_stderr.txt

echo "Response:"
cat test2_response.txt 2>/dev/null || echo "(no response)"

# Test 3: Full sequence
echo ""
echo "Test 3: Full sequence with tools/list"
{
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}'
    sleep 0.1
    echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
    sleep 0.1
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
    sleep 1
} | timeout 5s ./debug_server > test3_response.txt 2> test3_stderr.txt

echo "Response:"
cat test3_response.txt 2>/dev/null || echo "(no response)"

echo ""
echo "📋 Latest server logs:"
tail -10 debug_server.log 2>/dev/null || echo "(no logs)"

echo ""
echo "🔍 Detailed test completed!"
