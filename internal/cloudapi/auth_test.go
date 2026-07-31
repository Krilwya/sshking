package cloudapi

import "testing"

func TestValidateLoopbackRedirect(t *testing.T) {
	valid := []string{"http://127.0.0.1:41231/oauth/callback", "http://[::1]:61234/oauth/callback"}
	for _, value := range valid {
		if err := validateLoopbackRedirect(value); err != nil {
			t.Errorf("%s: %v", value, err)
		}
	}
	invalid := []string{"https://127.0.0.1:41231/oauth/callback", "http://localhost:41231/oauth/callback", "http://127.0.0.1/oauth/callback", "http://8.8.8.8:41231/oauth/callback", "http://127.0.0.1:41231/other", "http://127.0.0.1:41231/oauth/callback?next=x"}
	for _, value := range invalid {
		if err := validateLoopbackRedirect(value); err == nil {
			t.Errorf("expected %s to fail", value)
		}
	}
}

func TestPKCE(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceChallenge(verifier); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if !validChallenge(want) {
		t.Fatal("known challenge rejected")
	}
}
