package session

import (
	"context"
	"time"

	"github.com/open-fin/jiuwen-auth-adapter/internal/principal"
)

type Session struct {
	ID        string
	Principal principal.Principal
	ExpiresAt time.Time
}

type Store interface {
	Create(ctx context.Context, session Session) error
	Get(ctx context.Context, id string) (Session, error)
	Delete(ctx context.Context, id string) error
}
