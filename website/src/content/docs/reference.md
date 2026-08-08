---
title: Reference
description: CLI verbs, the HTTP surface, and the MCP tools an AI client can call.
---

## CLI

```sh
askrelay <verb> [flags]
```

| Verb | Status | What |
|---|---|---|
| `serve` | ✅ | run the relay (see [config](/self-host/#configuration-flags--env-only)) |
| `invite <email>` | ✅ | mint a single-use enrollment invite, print the invite URL |
| `version` | ✅ | print the build version |
| `help` | ✅ | list verbs |
| `inbox` / `approve` / `device` / `status` | ⏳ planned | human-facing operate verbs (with the CLI work package) |

Until the operate verbs land, enrollment is the `POST /enroll/{token}` call and
approvals happen through your AI client's MCP tools.

## HTTP surface

| Route | Auth | Purpose |
|---|---|---|
| `GET /healthz` | — | liveness + version |
| `POST /enroll/{token}` | invite token | enroll a device, return its credential |
| `GET /.well-known/oauth-protected-resource` | — | RFC 9728 resource metadata |
| `GET /.well-known/oauth-authorization-server` | — | RFC 8414 authorization-server metadata |
| `GET /oauth/jwks` | — | Ed25519 verification key (JWK Set) |
| `GET` / `POST /oauth/authorize` | device credential | authorization endpoint + login page |
| `POST /oauth/token` | PKCE | token endpoint (authorization_code, refresh_token) |
| `POST /oauth/register` | — | Dynamic Client Registration (RFC 7591) |
| `POST /mcp` | Bearer (access token) | Streamable HTTP MCP endpoint |
| `GET /ws` | device credential | delivery/approval push + outbound submit |

TLS is terminated by the reverse proxy in front of the relay.

## MCP tools

An authenticated client sees these tools. Every content-bearing response is
spotlighted (quarantined) — see [Concepts](/concepts/).

| Tool | What it does |
|---|---|
| `send_message` | start or continue a thread by sending a question/message |
| `check_inbox` | list threads with something awaiting you |
| `get_thread` | read a thread's messages and state |
| `approve_message` | approve an inbound message (the inbound gate) |
| `decline_message` | decline an inbound message |
| `approve_reply` | release a drafted reply (the outbound gate) |
| `discard_reply` | discard a drafted reply |
| `set_thread_grant` | grant or revoke standing per-thread approval for a direction |
| `wait_for_activity` | long-poll until there's activity, bounded by your client profile |

Read-only tools are annotated as such; timeouts on `wait_for_activity` are capped
per client profile.

## Configuration

All `serve` flags/env are in [Self-host & operate](/self-host/#configuration-flags--env-only).
