# Feature-Based Directory Structure

This document proposes a feature-based directory structure for the backend to improve maintainability, scalability, and ease of migration to microservices.

## Overview

Instead of grouping files by technical layer (e.g., `controllers`, `services`, `repositories` at the root), we group them by **Business Capability (Feature)**. Each feature becomes a self-contained module with its own domain logic, data access, and transport layer.

## Proposed Structure

We will move from a technical grouping to a feature grouping within `internal/`.

```
backend/
├── cmd/
│   └── api/
│       └── main.go          # Entry point, wires everything together
├── internal/
│   ├── shared/              # Shared kernel (logging, config, common errors)
│   ├── features/
│   │   ├── auth/            # Feature: User Identity & Auth
│   │   │   ├── domain/      # Entities (User)
│   │   │   ├── service/     # Business logic
│   │   │   ├── repository/  # Database interactions
│   │   │   └── transport/   # HTTP handlers
│   │   ├── conversations/   # Feature: Conversation Management
│   │   │   ├── ...
│   │   ├── location/        # Feature: Live Tracking
│   │   │   ├── ...
│   │   ├── websocket/       # Feature: Connection Handshake & Registry
│   │   │   ├── ...
│   │   └── messaging/       # Feature: Chat Messages & History
│   │       ├── ...
└── ...
```

## Detailed Feature Breakdown

Based on the documentation in `backend/docs/features`, we have identified 5 distinct features:

### 1. Auth Feature (`internal/features/auth`)
**Docs**: `docs/features/1-auth`
Responsible for user authentication and identity.
- **Entities**: `User`
- **Responsibilities**: Registration, Login, Password Hashing.

### 2. Conversations Feature (`internal/features/conversations`)
**Docs**: `docs/features/2-conversations`
Responsible for managing chat groups and direct pairs.
- **Entities**: `Conversation`, `ConversationMember`
- **Responsibilities**: Create conversations, Manage participants, Check membership.

### 3. Location Feature (`internal/features/location`)
**Docs**: `docs/features/3-live-locations`
Responsible for real-time location streaming.
- **Entities**: `LiveLocationSession`, `LiveLocationPoint`
- **Responsibilities**: Start/End sessions, Ephemeral location updates (Redis/WebSockets).

### 4. WebSocket Feature (`internal/features/websocket`)
**Docs**: `docs/features/4-ws-handshake`
Responsible for the raw connection layer and user binding.
- **Responsibilities**: Connection Upgrade, Handshake (identifying user), Connection Registry (tracking active sockets).

### 5. Messaging Feature (`internal/features/messaging`)
**Docs**: `docs/features/5-messaging`
Responsible for durable chat messages.
- **Entities**: `Message`, `DeliveryReceipt`
- **Responsibilities**: Send/Receive messages, Persistence, History retrieval, Read receipts.

## Benefits

1.  **High Cohesion**: Code related to the same feature stays together. For example, "Messaging" logic is isolated from "Location" logic.
2.  **Low Coupling**: Features communicate via defined interfaces.
3.  **Microservices Readiness**: Since each feature is already isolated in a folder with clear boundaries, extracting it to a separate microservice is straightforward.
    -   *Example*: To move "Location" to a separate service, you would extract the `internal/features/location` folder and wrap it with its own `main.go` and transport adaptors.

## Rules of Engagement

1.  **Dependencies**: Features should ideal not import each other directly. Use `internal/shared` for common types or Dependency Injection interfaces.
2.  **Shared Kernel**: Code shared by *all* features (e.g., Logger, Database connection, Middleware) goes into `internal/shared`.
3.  **Wiring**: The `cmd/api/main.go` is the composition root. It initializes repositories and services and injects them into the respective feature handlers.

## Refactoring Strategy

This section outlines the step-by-step strategy to migrate the current codebase (layered: `repo/`, `service/`, `transport/`) to the new feature-based structure.

### Phase 1: Preparation
1.  **Initialize Directory**: Create `internal/features/` and `internal/shared/`.
2.  **Shared Kernel**: Move common utilities (logging, config, basic errors) to `internal/shared/`.
3.  **No-Big-Bang**: We will refactor one feature at a time to keep the application buildable.

### Phase 2: Feature Migration (Example: Auth)
For the **Auth** feature (User domain), follow these steps:

1.  **Create Folders**:
    ```bash
    mkdir -p internal/features/auth/{domain,repository,service,transport}
    ```
2.  **Move Domain**:
    -   Extract `User` struct from `internal/domain/domain.go` to `internal/features/auth/domain/user.go`.
3.  **Move Repository**:
    -   Move `internal/repo/memory/user_repo.go` (and pg version) to `internal/features/auth/repository/`.
    -   *Note*: Update package name to `repository` or `authrepo`.
4.  **Move Service**:
    -   Move `internal/service/user_service.go` to `internal/features/auth/service/`.
    -   Update imports to point to the new domain/repository paths.
5.  **Move Transport**:
    -   Move user-related handlers from `internal/transport/httpserver` to `internal/features/auth/transport/`.
6.  **Fix Wiring**:
    -   Update `cmd/api/main.go` to import the new `auth` packages and wire them up.

### Phase 3: Rinse and Repeat
Repeat the process for:
-   **Conversations** (`Conversation`, `Member`)
-   **Location** (`LiveLocation*`)
-   **Messaging** (`Message`, `DeliveryReceipt`)
-   **WebSocket** (Handshake logic)

### Phase 4: Cleanup
1.  **Remove Empty Dirs**: Once all code is moved, delete the empty `internal/repo`, `internal/service`, `internal/transport`, and `internal/domain` directories.
2.  **Verify**: Run all tests to ensure no regressions.
