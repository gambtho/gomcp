# GoMCP v1.3.0 - MCP Specification Compliance Release

## 🎯 What's New

**100% MCP Specification Compliance** - This release ensures strict adherence to all three MCP protocol versions (draft, 2024-11-05, 2025-03-26).

## ✨ Key Improvements

- **✅ Fixed Prompt Content Format** - Now returns proper `{type: "text", text: "content"}` objects
- **✅ Standardized Parameters** - Changed `"variables"` to `"arguments"` in prompt operations  
- **✅ Correct Error Codes** - Implemented proper JSON-RPC error codes (-32602)
- **✅ Enhanced Logging** - Improved slog integration and debug output
- **✅ 100% Test Coverage** - All tests pass across all MCP versions

## 🔄 Breaking Changes

**Prompt Operations:**
```json
// Before: {"params": {"name": "greeting", "variables": {"name": "User"}}}
// After:  {"params": {"name": "greeting", "arguments": {"name": "User"}}}
```

**Response Format:**
```json
// Before: {"content": "Hello, User!"}
// After:  {"content": {"type": "text", "text": "Hello, User!"}}
```

## 🚀 Migration

1. Update prompt requests to use `"arguments"` instead of `"variables"`
2. Handle new object-based content format in responses
3. Remove any expectations for `kind` field in resource lists

## 📊 Stats

- **200+ test cases** all passing
- **3 MCP versions** fully supported
- **Multiple transports** (HTTP, WebSocket, SSE, stdio, UDP, Unix, MQTT, NATS, gRPC)
- **Zero breaking API changes** outside of specification compliance

---

**Full Release Notes**: [RELEASE_NOTES_v1.3.0.md](./RELEASE_NOTES_v1.3.0.md) 