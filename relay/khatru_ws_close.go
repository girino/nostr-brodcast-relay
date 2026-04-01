package relay

import (
	"context"
	"reflect"
	"unsafe"

	"github.com/fasthttp/websocket"
	"github.com/fiatjaf/khatru"
)

// closeKhatruWSOnRateLimit closes the websocket when event or filter rate limits fire, so the client
// must open a new connection instead of staying connected and hammering per-message rejects.
//
// khatru.WebSocket keeps conn unexported; we use reflect on field order matching
// github.com/fiatjaf/khatru v0.19.x (conn, mutex, Request, Context, cancel, ...).
func closeKhatruWSOnRateLimit(ctx context.Context) {
	ws := khatru.GetConnection(ctx)
	if ws == nil {
		return
	}
	rv := reflect.ValueOf(ws).Elem()
	if rv.NumField() < 5 {
		return
	}
	connField := rv.Field(0)
	if connField.Kind() != reflect.Ptr {
		return
	}
	pp := (**websocket.Conn)(unsafe.Pointer(connField.UnsafePointer()))
	if pp == nil || *pp == nil {
		return
	}
	c := *pp
	_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limited"))
	_ = c.Close()

	cancelField := rv.Field(4)
	if cancelField.Kind() == reflect.Func && cancelField.IsValid() && !cancelField.IsNil() {
		cancelField.Call(nil)
	}
}
