package relay

import (
	"context"

	"github.com/fasthttp/websocket"
	"github.com/fiatjaf/khatru"
	"github.com/girino/nostr-lib/logging"
)

// closeKhatruWSOnRateLimit sends a WebSocket close frame when event or filter rate limits fire so the
// client must reconnect. Uses khatru.WebSocket.WriteMessage (mutex-protected); we must not call
// *websocket.Conn methods directly — that races the read loop and can SIGSEGV (see fasthttp/websocket Conn.beginMessage).
func closeKhatruWSOnRateLimit(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			logging.DebugMethod("relay", "closeKhatruWSOnRateLimit", "recover after rate-limit close: %v", err)
		}
	}()
	ws := khatru.GetConnection(ctx)
	if ws == nil {
		return
	}
	payload := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limited")
	_ = ws.WriteMessage(websocket.CloseMessage, payload)
}
