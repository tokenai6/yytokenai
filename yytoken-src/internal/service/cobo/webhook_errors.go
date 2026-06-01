package cobo

import "errors"

var ErrWebhookDeny = errors.New("cobo webhook deny")

func IsWebhookDenyError(err error) bool {
	return errors.Is(err, ErrWebhookDeny)
}
