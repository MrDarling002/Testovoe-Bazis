package email

import "context"

type Sender interface {
	SendInvitation(ctx context.Context, email string, teamName string, inviterName string) error
}
