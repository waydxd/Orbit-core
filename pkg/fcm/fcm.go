package fcm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// ErrInvalidToken is a sentinel error that callers (including tests) can use to signal
// that the FCM token is no longer valid and should be removed.
var ErrInvalidToken = errors.New("fcm: invalid or unregistered token")

// Client wraps the Firebase messaging client.
type Client struct {
	msg *messaging.Client
}

var (
	instance *Client
	once     sync.Once
	initErr  error
)

// Init initialises the Firebase App singleton using raw service-account JSON.
// Subsequent calls are no-ops; the same instance is always returned.
func Init(ctx context.Context, credentialsJSON string) (*Client, error) {
	once.Do(func() {
		if credentialsJSON == "" {
			initErr = fmt.Errorf("FIREBASE_CREDENTIALS_JSON is empty; FCM notifications will not be sent")
			return
		}

		app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(credentialsJSON)))
		if err != nil {
			initErr = fmt.Errorf("firebase.NewApp: %w", err)
			return
		}

		msgClient, err := app.Messaging(ctx)
		if err != nil {
			initErr = fmt.Errorf("firebase app.Messaging: %w", err)
			return
		}

		instance = &Client{msg: msgClient}
	})

	return instance, initErr
}

// SendNotification sends an FCM notification to a single device token.
// dataPayload is forwarded as FCM data fields (key/value strings).
func (c *Client) SendNotification(ctx context.Context, token, title, body string, dataPayload map[string]string) error {
	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: dataPayload,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	_, err := c.msg.Send(ctx, msg)
	return err
}

// IsInvalidToken reports whether the FCM error indicates a stale or unregistered token
// that should be removed from the database. Only token-not-registered errors are treated
// as invalid tokens; other argument errors may relate to the payload and should not trigger
// token deletion.
func IsInvalidToken(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrInvalidToken) {
		return true
	}
	return messaging.IsRegistrationTokenNotRegistered(err)
}
