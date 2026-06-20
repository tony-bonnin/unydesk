# UnyDesk (Alpha)

**Remote control plane in Go, separated from UnyPort**

`Go` `Single Binary` `Single Port` `Alpine-first` `musl-friendly` `API-first`

UnyDesk is a standalone project for remote access and control.
It is intentionally separated from UnyPort so the remote stack can evolve on its own,
with a clean API contract that UnyPort can consume later.

## Goals

- separate runtime from UnyPort
- keep Alpine Linux and musl as first-class constraints
- start with an API-first broker before capture and input drivers
- prefer pure Go where possible
- keep the door open for small native adapters only where unavoidable

## Target architecture

- **Broker**: Go HTTP server for auth, session lifecycle, signaling, and health
- **Agent**: future host-side process for capture, input injection, and relay
- **Web client**: future browser UI for session initiation and control
- **UnyPort integration**: later, through HTTP API instead of code coupling

## Why separate it

UnyPort is a monitoring and operations portal.
UnyDesk has a different performance, security, and protocol surface.
Keeping them separate lets us:

- ship remote-control faster without destabilizing UnyPort
- harden the protocol independently
- test Alpine and musl constraints in isolation
- choose the best transport path for remote sessions

## Current scope

This initial scaffold provides:

- a standalone Go backend
- YAML-based settings
- a health endpoint
- a small session broker API
- Docker development flow aligned with Alpine

It does **not** yet provide:

- screen capture
- keyboard/mouse injection
- WebRTC signaling
- TURN/STUN integration
- host agent

## API preview

- `GET /healthz`
- `GET /api/v1/info`
- `GET /api/v1/sessions`
- `POST /api/v1/sessions`
- `GET /api/v1/sessions/:id`
- `POST /api/v1/sessions/:id/offer`
- `POST /api/v1/sessions/:id/answer`
- `POST /api/v1/sessions/:id/candidates`
- `POST /api/v1/sessions/:id/close`

## Development

```sh
cd docker_undesk
docker compose up --build
```

Default URL:

```text
http://127.0.0.1:8890
```

## Suggested next steps

1. Add a lightweight agent process for Alpine hosts.
2. Add WebRTC signaling based on Pion.
3. Add X11-first capture and input adapters.
4. Plug UnyPort into this API only after the protocol stabilizes.
