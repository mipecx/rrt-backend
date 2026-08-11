package sms

import (
	"context"
	"log/slog"
)

type LogProvider struct {
	log *slog.Logger
}

func NewDevProvider(log *slog.Logger) *LogProvider {
	return &LogProvider{
		log: log,
	}
}

func (p *LogProvider) Send(ctx context.Context, to, body string) error {
	p.log.Info("SMS sent (dev mode)", "to", to, "body", body)
	return nil
}
