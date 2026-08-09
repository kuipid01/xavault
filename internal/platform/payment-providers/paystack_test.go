package paymentproviders

import "testing"

func TestPrependPaystackTestBank(t *testing.T) {
	banks := []PaystackBank{{Code: "058", Name: "GTBank"}}
	got := prependPaystackTestBank(banks)
	if len(got) != 2 || got[0].Code != "001" {
		t.Fatalf("expected test bank first, got %+v", got)
	}

	got = prependPaystackTestBank(got)
	if len(got) != 2 {
		t.Fatalf("test bank was duplicated: %+v", got)
	}
}
