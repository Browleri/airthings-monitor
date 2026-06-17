package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ntfy delivers push notifications via an ntfy.sh-compatible server.
// Subscribe to the configured topic in the ntfy mobile app to receive alerts.
type Ntfy struct {
	url    string
	client *http.Client
}

func NewNtfy(url string) *Ntfy {
	return &Ntfy{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a push notification. priority is an ntfy priority string
// ("max", "high", "default", "low", "min"). tags is a comma-separated list
// of ntfy tag names (e.g. "warning,house").
func (n *Ntfy) Notify(ctx context.Context, title, message, priority, tags string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, strings.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if title != "" {
		req.Header.Set("X-Title", title)
	}
	if priority != "" {
		req.Header.Set("X-Priority", priority)
	}
	if tags != "" {
		req.Header.Set("X-Tags", tags)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned %d", resp.StatusCode)
	}
	return nil
}
