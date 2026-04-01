package ratelimit

import (
	"context"

	"github.com/fasthttp/websocket"
	"github.com/fiatjaf/khatru"
)

// CloseWebSocket sends a policy-violation close frame on the current khatru connection.
// Uses WebSocket.WriteMessage (mutex-safe). reason must be non-empty.
func CloseWebSocket(ctx context.Context, reason string, onPanic func(recovered any)) {
	if reason == "" {
		reason = "rate limited"
	}
	defer func() {
		if err := recover(); err != nil {
			if onPanic != nil {
				onPanic(err)
			}
		}
	}()
	ws := khatru.GetConnection(ctx)
	if ws == nil {
		return
	}
	payload := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason)
	_ = ws.WriteMessage(websocket.CloseMessage, payload)
}
