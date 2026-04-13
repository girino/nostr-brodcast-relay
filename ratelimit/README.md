# ratelimit — khatru rate limits, warnings, optional close, and optional IP ban

This package wires [khatru](https://github.com/fiatjaf/khatru) `RejectConnection`, `RejectEvent`, and `RejectFilter` hooks so you get:

- **Token-bucket limits** per client IP (connection upgrades, published events, REQ filters), using khatru’s built-in policy limiters.
- **Progressive warnings**: the first *N* rate-limit rejects for a client still return a normal reject message, with an extra suffix explaining that the connection will close on the next hit.
- **Forced WebSocket close** on the next rate-limit hit after those warnings (close reason is configurable, default `rate limited`).
- **Optional soft-only mode**: disable forced close behavior so event/filter rejects always stay soft (no close, no ban state).
- **Optional escalating IP ban**: after a forced close, that client IP cannot open **new** WebSocket upgrades for a configurable base duration. When the ban ends, the IP enters **probation** for `lastBanDuration × ProbationMultiplier` (default multiplier **1** = same length as the ban). If another forced close happens during probation, the next ban is `lastBanDuration × RepeatOffenderMultiplier` (default **2**), capped by `MaxBanDuration`. Completing probation without a violation resets escalation back to the base ban.

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
    SoftRejectCount:      3,     // optional; default 3 if ≤ 0
    DisableDisconnect:    false, // optional; when true all rejects stay soft and bans are disabled
    BaseBanDuration:            1 * time.Minute,
    MaxBanDuration:             24 * time.Hour, // optional; default 24h if ≤ 0 after normalize
    ProbationMultiplier:        1,              // optional; default 1
    RepeatOffenderMultiplier:   2,              // optional; default 2
    CloseReason:                "rate limited", // optional; this is the default
    LogFilePath:                "/var/log/relay/rate-limit.jsonl", // optional JSONL audit log
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
| `DisableDisconnect` | Keep event/filter rejects soft-only. When `true`, sockets are never force-closed by this package and IP bans are disabled. |
| `BaseBanDuration` | Length of the **first** ban after a forced close, and again after probation completes cleanly. `0` disables banning (strikes/close still apply if event/filter limits are enabled). |
| `MaxBanDuration` | Upper bound for ban length when probation is broken (each break uses `min(previousBan × RepeatOffenderMultiplier, MaxBanDuration)`). Normalized default `24h` if unset or `≤ 0`. |
| `ProbationMultiplier` | Probation length after a ban ends: `previousBan × ProbationMultiplier`. **`0` disables probation** (no window after a ban; escalation state clears when the ban ends). Omit or set explicitly (e.g. `1`) for the default behavior. |
| `RepeatOffenderMultiplier` | Next ban after breaking probation: `previousBan × RepeatOffenderMultiplier` (then capped). **`0` disables escalation** (next ban is `BaseBanDuration`). Omit or set explicitly (e.g. `2`) for exponential bans. |
| `CloseReason` | WebSocket close payload (policy violation). Empty string becomes `rate limited`. |
| `LogFilePath` | Optional path to append JSONL audit entries for all rate-limit decisions (connection/event/filter rejects, forced closes, ban checks). Includes request metadata (headers/cookies) and event/filter payloads when available. |
| `LogDebug` | Optional; called for debug-style messages (rejects, warnings, closes, bans). |
| `OnPanic` | Optional; if closing the WebSocket panics, this receives the recovered value (library also recovers so your process keeps running). |

## Strike key and bans

- Strikes are keyed by **client IP** when khatru provides one; otherwise by a synthetic key `ws:%p` for the current connection.
- **Ban / probation** only apply when a real client IP is known; `BaseBanDuration > 0` with an empty IP does not add ban state.
- If a forced close happens while a ban is **still active** (e.g. a second open socket from the same IP), the existing `banUntil` is not shortened.

### Escalation timeline (example: base 1m, max 24h, probation ×1, repeat ×2)

1. Forced close → ban **1m** (no new upgrades until it ends).
2. Ban ends → probation **1m** (upgrades allowed; length = last ban × probation multiplier).
3. Forced close in probation → next ban **2m** (= 1m × 2), then probation **2m** (= 2m × 1).
4. Clean probation (no forced close) → next offense is **1m** again.
5. Repeat: each probation break multiplies the previous ban by repeat multiplier until **max** (e.g. 1m → 2m → 4m → … → 24h with ×2).

## Hook order

`Apply` mutates the relay’s policy slices in place:

- If `BaseBanDuration > 0`, a **connection** reject that enforces the active ban is **prepended** to `RejectConnection` (runs first).
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
