#!/bin/bash

echo "🔍 Interactive stdio MCP debug test..."
echo "======================================"

echo "📦 Server already built"

echo "🚀 Testing with proper MCP client simulation..."

# Test with a more realistic client interaction
{
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"debug-client","version":"1.0.0"}}}'
    sleep 0.5
    echo '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
    sleep 0.5
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
    sleep 1
} | ./debug_server > interactive_output.txt 2> interactive_stderr.txt

echo "📥 Server responses:"
if [ -s interactive_output.txt ]; then
    cat interactive_output.txt
else
    echo "(no responses)"
fi

echo ""
echo "📋 Server stderr during interactive test:"
if [ -s interactive_stderr.txt ]; then
    cat interactive_stderr.txt
else
    echo "(no stderr)"
fi

echo ""
echo "📋 Server application logs:"
if [ -s debug_server.log ]; then
    echo "All logs:"
    cat debug_server.log
else
    echo "(no log file)"
fi

echo ""
echo "🔍 Interactive test completed!"
