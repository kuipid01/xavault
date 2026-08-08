package alerts

import "testing"

func TestSanitizeMetadataRedactsSensitiveValues(t *testing.T) {
	clean := sanitizeMetadata(map[string]interface{}{"reference": "DEP-1", "access_token": "secret", "pin": "1234", "payload": "raw"})
	if clean["reference"] != "DEP-1" {
		t.Fatal("safe reference was removed")
	}
	for _, key := range []string{"access_token", "pin", "payload"} {
		if clean[key] != "[REDACTED]" {
			t.Fatalf("%s was not redacted", key)
		}
	}
}
