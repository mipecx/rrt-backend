package sms

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTwilioProvider_Send(t *testing.T) {
	var gotPath, gotAuth string
	var gotForm url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseForm()
		gotForm = r.PostForm
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	p := NewTwilioSMSProvider("AC123", "secret", "+15000000000")
	p.endpoint = server.URL + "/2010-04-01/Accounts/AC123/Messages.json"

	err := p.Send(context.Background(), "+15550000000", "hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotPath != "/2010-04-01/Accounts/AC123/Messages.json" {
		t.Errorf("wrong path: %s", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("no basic auth, got: %q", gotAuth)
	}
	if gotForm.Get("To") != "+15550000000" || gotForm.Get("Body") != "hello" {
		t.Errorf("wrong form: %v", gotForm)
	}
}
