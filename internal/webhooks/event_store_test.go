package webhooks

import "testing"

func TestNewPaystackEventUsesResourceIdentity(t *testing.T) {
	payload := []byte(`{"event":"transfer.success","data":{"id":42,"reference":"wd-1","status":"success"}}`)
	event, err := NewPaystackEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if event.ProviderEventID != "paystack:transfer.success:42" {
		t.Fatalf("unexpected provider event id: %s", event.ProviderEventID)
	}
}

func TestNewPaystackEventSeparatesReversalFromSuccess(t *testing.T) {
	success, _ := NewPaystackEvent([]byte(`{"event":"transfer.success","data":{"reference":"wd-1","status":"success"}}`))
	reversed, _ := NewPaystackEvent([]byte(`{"event":"transfer.reversed","data":{"reference":"wd-1","status":"reversed"}}`))
	if success.ProviderEventID == reversed.ProviderEventID {
		t.Fatal("success and reversal must not share a deduplication identity")
	}
}

func TestNewPaystackEventRejectsMissingType(t *testing.T) {
	if _, err := NewPaystackEvent([]byte(`{"data":{"id":1}}`)); err == nil {
		t.Fatal("expected missing event type to fail")
	}
}
