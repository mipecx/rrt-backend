package service

import (
	"context"
)

type SMSProvider interface {
	Send(ctx context.Context, to, body string) error
}
