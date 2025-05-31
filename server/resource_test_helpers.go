package server

// Test helpers for exposing internal functions for testing

// TestEnsureContentsArray exposes ensureContentsArray for testing
func TestEnsureContentsArray(response map[string]interface{}, uri string) map[string]interface{} {
	return ensureContentsArray(response, uri)
}

// TestEnsureValidContentItems exposes ensureValidContentItems for testing
func TestEnsureValidContentItems(items []interface{}) []interface{} {
	return ensureValidContentItems(items)
}
