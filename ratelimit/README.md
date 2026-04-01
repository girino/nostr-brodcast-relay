# ratelimit — khatru rate limits, warnings, close, and IP ban

This package wires [khatru](https://github.com/fiatjaf/khatru) `RejectConnection`, `RejectEvent`, and `RejectFilter` hooks so you get:

- **Token-bucket limits** per client IP (connection upgrades, published events, REQ filters), using khatru’s built-in policy limiters.
- **Progressive warnings**: the first *N* rate-limit rejects for a client still return a normal reject message, with an extra suffix explaining that the connection will close on the next hit.
- **Forced WebSocket close** on the next rate-limit hit after those warnings (close reason is configurable, default `rate limited`).
- **Optional temporary ban**: after a forced close, that client IP can be blocked from **new** WebSocket upgrades for a duration you choose (existing connections are not kicked).

Closing uses `WebSocket.WriteMessage` on the khatru connection (not a raw `*websocket.Conn`), which matches how khatru serializes writes.

## Install

From another module that already uses khatru:

```bash
go get github.com/girino/nostr-brodcast-relay@latest
```

Import the subpackage:

```go
import "github.com/girino/nostr-brodcast-relay/ratelimit"
```

You will pull this repo as a dependency; only import `ratelimit` (and transitively khatru, go-nostr, etc.).

## Minimal usage

Build a `ratelimit.Config`, create a manager with `New`, then call `Apply` **once** on your `*khatru.Relay` (after you construct the relay, typically where you attach other policies):

```go
relay := khatru.NewRelay()

ratelimit.New(ratelimit.Config{
    Connection: ratelimit.Bucket{
        Tokens:   5,
        Interval: time.Minute,
        Max:      30,
    },
    EventIP: ratelimit.Bucket{
        Tokens:   2,
        Interval: 3 * time.Minute,
        Max:      10,
    },
    FilterIP: ratelimit.Bucket{
        Tokens:   20,
        Interval: time.Minute,
        Max:      100,
    },
    SoftRejectCount: 3, // optional; default 3 if ≤ 0
    BanDuration:     5 * time.Minute,
    CloseReason:     "rate limited", // optional; this is the default
    LogDebug: func(format string, args ...any) {
        log.Printf("[ratelimit] "+format, args...)
    },
}).Apply(relay)
```

Any bucket with `Tokens`, `Interval`, and `Max` all positive is **enabled**; zero or unset fields disable that limiter (`Bucket.Enabled()`).

## Configuration

| Field | Meaning |
|--------|--------|
| `Connection` | Limit **new** HTTP/WebSocket upgrades per IP (khatru `ConnectionRateLimiter`). No soft-strike path; rejects are immediate. |
| `EventIP` | Limit published **events** per IP; shares strike counter with `FilterIP` for the same client. |
| `FilterIP` | Limit **REQ** filters per IP; same strike counter as `EventIP` per client key. |
| `SoftRejectCount` | Number of **soft** rate-limit responses (warning suffix only) before the **next** reject also closes the socket. Default `3` → strikes 1–3 warn, 4th closes. |
| `BanDuration` | After a forced close, block **new** connections from that IP for this duration. `0` disables banning (strikes/close still apply if event/filter limits are enabled). |
| `CloseReason` | WebSocket close payload (policy violation). Empty string becomes `rate limited`. |
| `LogDebug` | Optional; called for debug-style messages (rejects, warnings, closes, bans). |
| `OnPanic` | Optional; if closing the WebSocket panics, this receives the recovered value (library also recovers so your process keeps running). |

## Strike key and bans

- Strikes are keyed by **client IP** when khatru provides one; otherwise by a synthetic key `ws:%p` for the current connection.
- **Temporary ban** only applies when a real IP is known; `BanDuration > 0` with an empty IP does not add a ban entry.

## Hook order

`Apply` mutates the relay’s policy slices in place:

- If `BanDuration > 0`, a **connection** reject that enforces the ban is **prepended** to `RejectConnection` (runs first).
- Connection / event / filter limiters are **appended** to the respective slices.

If you already set `RejectConnection` / `RejectEvent` / `RejectFilter`, call `Apply` in the order you want those hooks to run relative to this package (e.g. call `Apply` after attaching checks that should run first, or before checks that should run after the built-in limiters).

## Standalone close helper

If you need the same close behavior outside these hooks:

```go
ratelimit.CloseWebSocket(ctx, "rate limited", func(rec any) {
    log.Printf("close panic: %v", rec)
})
```

`ctx` must be the request/context khatru uses for the active WebSocket (same as inside `RejectEvent` / `RejectFilter`).

## License

Same as the parent repository.
