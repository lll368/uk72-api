package main

import (
	"net/http"
	"testing"
)

func TestSessionCookieUsesLaxSameSiteForExternalPaymentReturns(t *testing.T) {
	options := sessionCookieOptions()

	if options.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax, got %v", options.SameSite)
	}
}
