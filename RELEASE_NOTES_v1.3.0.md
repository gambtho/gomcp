# GoMCP v1.3.0 Release Notes

**Release Date**: May 28, 2025  
**Previous Version**: v1.2.2  
**New Version**: v1.3.0

## 🎯 Overview

This release focuses on **100% MCP specification compliance** across all three supported protocol versions (draft, 2024-11-05, and 2025-03-26). We've implemented comprehensive fixes to ensure strict adherence to the Model Context Protocol specifications, improved logging infrastructure, and enhanced test coverage.

## ✨ Major Features & Improvements

### 🔧 **Complete MCP Specification Compliance**

- **Prompt Content Format**: Fixed prompt responses to use proper object structure with `type` and `text` fields as required by all MCP specifications
- **Parameter Standardization**: Corrected parameter names from `"variables"` to `"arguments"` across all prompt operations
- **Error Code Compliance**: Implemented correct JSON-RPC error codes (-32602 for invalid parameters) as specified
- **Resource Format Validation**: Removed non-standard `kind` field expectations from resource lists
- **Variable Substitution**: Enhanced prompt variable handling to be more lenient with missing variables while maintaining specification compliance

### 📊 **Enhanced Test Coverage**

- **100% Test Pass Rate**: All tests now pass across the entire codebase
- **Cross-Version Testing**: Comprehensive test coverage for draft, 2024-11-05, and 2025-03-26 specifications
- **Specification Adherence**: Tests now strictly validate against actual MCP specification requirements
- **Resource Template Testing**: Improved separation between regular resources and resource templates

### 🔍 **Improved Logging Infrastructure**

- **Enhanced slog Integration**: Better structured logging throughout the codebase
- **Debug Information**: Improved debug output for troubleshooting transport and protocol issues
- **Error Context**: More detailed error messages with proper context for debugging

## 🐛 Bug Fixes

### **Prompt System Fixes**
- Fixed prompt content format to return proper `{type: "text", text: "content"}` objects instead of plain strings
- Corrected parameter name from `"variables"` to `"arguments"` in prompt requests
- Fixed error handling to return `-32602` (Invalid params) for prompt not found scenarios
- Enhanced variable substitution to leave placeholders unchanged when variables are missing

### **Resource System Fixes**
- Removed incorrect `kind` field expectations from resource list responses (not part of MCP specification)
- Fixed resource template exclusion from regular resource lists
- Improved resource content formatting for different MCP versions

### **Test Infrastructure Fixes**
- Updated all test files to use correct MCP method names and parameter formats
- Fixed test expectations to match actual specification requirements
- Resolved resource template handling in test scenarios

### **Documentation Fixes**
- Updated examples README to use `prompts/get` instead of incorrect `prompts/render`
- Fixed parameter names in JSON examples from `"args"` to `"arguments"`
- Ensured all documentation aligns with actual implementation

## 🔄 Breaking Changes

### **Parameter Name Changes**
- **Prompt Operations**: Parameter name changed from `"variables"` to `"arguments"` in prompt requests
  ```json
  // Before (v1.2.2)
  {"method": "prompts/get", "params": {"name": "greeting", "variables": {"name": "User"}}}
  
  // After (v1.3.0)
  {"method": "prompts/get", "params": {"name": "greeting", "arguments": {"name": "User"}}}
  ```

### **Response Format Changes**
- **Prompt Content**: Prompt responses now return proper content objects instead of plain strings
  ```json
  // Before (v1.2.2)
  {"content": "Hello, User!"}
  
  // After (v1.3.0)
  {"content": {"type": "text", "text": "Hello, User!"}}
  ```

## 📈 Performance & Quality Improvements

- **Test Execution**: Faster and more reliable test execution across all transport types
- **Error Handling**: More precise error responses with correct JSON-RPC error codes
- **Memory Usage**: Improved resource handling and cleanup
- **Code Quality**: Enhanced type safety and validation throughout the codebase

## 🔧 Technical Details

### **Specification Compliance Matrix**
| Feature | Draft | 2024-11-05 | 2025-03-26 | Status |
|---------|-------|-------------|-------------|---------|
| Prompt Content Format | ✅ | ✅ | ✅ | Fixed |
| Parameter Names | ✅ | ✅ | ✅ | Fixed |
| Error Codes | ✅ | ✅ | ✅ | Fixed |
| Resource Lists | ✅ | ✅ | ✅ | Fixed |
| Variable Substitution | ✅ | ✅ | ✅ | Enhanced |

### **Test Coverage**
- **Total Tests**: 200+ test cases
- **Pass Rate**: 100%
- **Coverage Areas**: All transport types, all MCP versions, all protocol operations
- **Integration Tests**: Cross-version compatibility, network conditions, concurrent operations

## 🚀 Migration Guide

### **For Existing Users**

1. **Update Prompt Requests**: Change `"variables"` to `"arguments"` in prompt operation parameters
2. **Handle New Content Format**: Update prompt response parsing to handle object-based content format
3. **Remove Kind Field**: Remove any expectations for `kind` field in resource list responses
4. **Test Your Integration**: Run comprehensive tests to ensure compatibility with new specification compliance

### **Code Examples**

**Prompt Request Migration:**
```go
// Before
params := map[string]interface{}{
    "name": "greeting",
    "variables": map[string]interface{}{"name": "User"},
}

// After
params := map[string]interface{}{
    "name": "greeting", 
    "arguments": map[string]interface{}{"name": "User"},
}
```

**Response Handling Migration:**
```go
// Before
content := response["content"].(string)

// After  
contentObj := response["content"].(map[string]interface{})
content := contentObj["text"].(string)
```

## 🙏 Acknowledgments

This release represents a significant step forward in MCP specification compliance and overall code quality. Special thanks to the community for reporting specification discrepancies and helping improve the library's adherence to protocol standards.

## 📚 Additional Resources

- [MCP Specification Documentation](./docs/spec-reference/README.md)
- [Migration Guide](./docs/migration/v1.2.2-to-v1.3.0.md)
- [API Reference](./docs/api-reference/README.md)
- [Examples](./examples/README.md)

---

**Full Changelog**: [v1.2.2...v1.3.0](https://github.com/localrivet/gomcp/compare/v1.2.2...v1.3.0) 