package cliproxyconfig

import "testing"

func TestRuntimeAuthIDIncludesProxyURL(t *testing.T) {
	withoutProxy := RuntimeAuthID("OpenCode-Go", "api-key", "https://opencode.ai/zen/go/v1", "")
	withProxy := RuntimeAuthID("OpenCode-Go", "api-key", "https://opencode.ai/zen/go/v1", "http://proxy.local")
	if withoutProxy == withProxy {
		t.Fatal("RuntimeAuthID() ignored proxy URL")
	}
	if len(withProxy) <= len("openai-compatibility:opencode-go:") {
		t.Fatalf("RuntimeAuthID() = %q, missing stable hash", withProxy)
	}
}
