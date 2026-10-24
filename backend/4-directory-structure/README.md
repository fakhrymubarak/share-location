 # Feature-Based Directory Structure

This document defines the feature-based architecture for the backend. By grouping code by feature rather than technical layer, we improve maintainability and facilitate future migration to microservices.

## Overview

We adopt a **Package-by-Feature** approach. Each major domain feature is a self-contained package within `internal/`.

## Feature Modules

The following features have been identified from the requirements:

### 1. Auth (`internal/auth/`)
Handles user identity and access control.
- **Responsibilities**: Registration, Login, Token generation/validation.
- **Components**:
  - `handler.go`: HTTP endpoints (`/register`, `/login`).
  - `service.go`: Password hashing (Bcrypt), business rules.
  - `repo.go`: User persistence (`users` table).
  - `model.go`: User structs.

### 2. Conversations (`internal/conversation/`)
Manages chat groups and memberships.
- **Responsibilities**: Create conversation, add/remove members, list conversations.
- **Components**:
  - `handler.go`: HTTP endpoints (`/conversations`).
  - `service.go`: Membership logic.
  - `repo.go`: `conversations` and `conversation_members` tables.

### 3. Live Location (`internal/location/`)
Real-time location sharing via WebSocket/Redis.
- **Responsibilities**: Publish location, Subscribe to updates, Session TTL.
- **Components**:
  - `handler.go`: WS handlers for location events.
  - `service.go`: Rate limits, Redis TTL management.
  - `repo.go`: Redis `live:*` keys, Postgres `live_location_sessions`.

### 4. Messaging (`internal/messaging/`)
Chat message persistence and history.
- **Responsibilities**: Send/Receive messages, History pagination, Delivery receipts.
- **Components**:
  - `handler.go`: WS handlers (`SEND_CHAT`), HTTP logic (`/messages`).
  - `service.go`: Fan-out logic, storage orchestration.
  - `repo.go`: `messages` table, Redis Pub/Sub for fan-out.

### 5. Session / Handshake (`internal/session/`)
Manages the WebSocket connection lifecycle and user presence.
- **Responsibilities**: WS Handshake, Connection Registry, Heartbeats.
- **Components**:
  - `handler.go`: WebSocket upgrade endpoint (`/ws`).
  - `registry.go`: In-memory map of active connections.
  - `service.go`: User upsert on handshake (legacy compatibility).

## Shared Kernel (`internal/shared/`)
Contains code used by multiple features:
- `db/`: Database connection factory.
- `json/`: JSON utilities.
- `logger/`: Structured logging.
- `errors/`: Standard error types.

## Directory Structure Example

```
backend/
├── cmd/
│   └── server/         # Main entry point wiring all features
├── internal/
│   ├── auth/           # Feature: Auth
│   ├── conversation/   # Feature: Conversations
│   ├── location/       # Feature: Live Locations
│   ├── messaging/      # Feature: Messaging
│   ├── session/        # Feature: WS Handshake & Registry
│   └── shared/         # Shared utilities
└── docs/               # Documentation
```

## Migration Strategy
1.  **Auth**: Implement `internal/auth` first (foundation).
2.  **Session**: Implement `internal/session` (WebSocket foundation).
3.  **Features**: Migrate `conversation`, `messaging`, `location` independently.
