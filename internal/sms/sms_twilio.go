package sms

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TwilioProvider struct {
	accountSID    string
	accountSecret string
	fromPhone     string
	httpClient    *http.Client
	endpoint      string
}

func NewTwilioSMSProvider(accountSID string, accountSecret string, fromPhone string) *TwilioProvider {
	return &TwilioProvider{
		accountSID:    accountSID,
		accountSecret: accountSecret,
		fromPhone:     fromPhone,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		endpoint:      "https://api.twilio.com/2010-04-01/Accounts/" + accountSID + "/Messages.json",
	}
}

func (p *TwilioProvider) Send(ctx context.Context, to, body string) error {
	form := url.Values{}
	form.Set("To", to)
	form.Set("From", p.fromPhone)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, p.endpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.accountSID, p.accountSecret)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("twilio returned status %d", resp.StatusCode)
	}

	return nil
}
