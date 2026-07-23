package domains

import "testing"

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantResult string
		wantValid  bool
	}{
		// Valid hostnames
		{name: "simple two-label", input: "example.com", wantResult: "example.com", wantValid: true},
		{name: "three labels", input: "sub.example.com", wantResult: "sub.example.com", wantValid: true},
		{name: "with hyphens", input: "my-app.example.com", wantResult: "my-app.example.com", wantValid: true},
		{name: "numeric label", input: "123.example.com", wantResult: "123.example.com", wantValid: true},
		{name: "mixed case normalized", input: "App.Example.COM", wantResult: "app.example.com", wantValid: true},
		{name: "max label 63 chars", input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com", wantResult: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com", wantValid: true},
		{name: "with whitespace trimmed", input: "  example.com  ", wantResult: "example.com", wantValid: true},

		// Invalid hostnames
		{name: "empty string", input: "", wantResult: "", wantValid: false},
		{name: "single label no dot", input: "localhost", wantResult: "", wantValid: false},
		{name: "leading hyphen in label", input: "-example.com", wantResult: "", wantValid: false},
		{name: "trailing hyphen in label", input: "example-.com", wantResult: "", wantValid: false},
		{name: "underscore in label", input: "my_app.example.com", wantResult: "", wantValid: false},
		{name: "space in hostname", input: "my app.example.com", wantResult: "", wantValid: false},
		{name: "empty label", input: ".example.com", wantResult: "", wantValid: false},
		{name: "double dot", input: "example..com", wantResult: "", wantValid: false},
		{name: "label over 63 chars", input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com", wantResult: "", wantValid: false},
		{name: "total over 253 chars", input: generateLongHostname(254), wantResult: "", wantValid: false},
		{name: "special characters", input: "ex@mple.com", wantResult: "", wantValid: false},
		{name: "trailing dot only", input: ".", wantResult: "", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, valid := ValidateHostname(tt.input)
			if valid != tt.wantValid {
				t.Errorf("ValidateHostname(%q) valid = %v, want %v", tt.input, valid, tt.wantValid)
			}
			if result != tt.wantResult {
				t.Errorf("ValidateHostname(%q) result = %q, want %q", tt.input, result, tt.wantResult)
			}
		})
	}
}

// generateLongHostname creates a hostname of approximately the given total length.
func generateLongHostname(totalLen int) string {
	// Build a hostname with labels of "aaa." repeated to reach target length
	result := ""
	for len(result) < totalLen-4 {
		result += "aaa."
	}
	result += "com"
	if len(result) < totalLen {
		// Pad the first label
		padding := totalLen - len(result)
		extra := ""
		for i := 0; i < padding; i++ {
			extra += "a"
		}
		result = extra + result
	}
	return result
}
