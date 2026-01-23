# End-to-End Sequences

- Status: Approved
- Owner: Backend Team
- Last updated: 23/01/2026

Sequence diagrams for the main app screens: login, conversation list, creating a new chat, and chatting with messages + live location.

## Login Screen
```mermaid
sequenceDiagram
    autonumber
    actor Client as Mobile/Web
    participant API as HTTP API
    participant DB as Postgres

    Client->>API: POST /v1/login {username}
    API->>DB: Upsert user by username
    DB-->>API: user_id, username
    API-->>Client: 200 {user_id, username}
    Client->>API: GET /ws?username=...
    API-->>Client: event CONNECTED {user_id, username}
    Note over Client: Client cached user_id locally to avoid re-login on refresh or reopen.
```

## Conversation Screen (List)
```mermaid
sequenceDiagram
    autonumber
    actor Client as Mobile/Web
    participant API as HTTP API
    participant DB as Postgres

    Client->>API: GET /v1/conversations?user_id=<uid>&type=all&limit=50
    API->>DB: SELECT conversations JOIN conversation_members WHERE user_id=<uid> ORDER BY created_at DESC
    DB-->>API: rows with member_ids
    API-->>Client: 200 [{id,type,title,member_ids,...}]
    Note over Client: Client caches conversations list locally (by updated_at/created_at) to avoid refetch on tab switch and invalidates when RECEIVE_CHAT for unknown conversation_id arrives.
    
```

## Create New Chat (from Conversation Screen)
```mermaid
sequenceDiagram
    autonumber
    actor Client as Mobile/Web
    participant API as HTTP API
    participant DB as Postgres

    Client->>API: POST /v1/conversations {type:"direct|group", title?, usernames:[...]}
    API->>DB: Upsert users
    API->>DB: Insert/find conversation (direct pair uniqueness enforced)
    API->>DB: Insert conversation_members (batch)
    DB-->>API: conversation + members
    API-->>Client: 200 {id,type,title,direct_pair_key,member_ids,created_at}
    Note over Client: Client updates cached conversation list and primes member cache from response.
```

## Chat Detail Screen (Send Chat + Live Location)
```mermaid
sequenceDiagram
    autonumber
    actor Client as Mobile/Web
    participant WS as WebSocket (Server)
    participant Service as Messaging/LiveLocation Services
    participant DB as Postgres
    participant Redis as Redis Pub/Sub
    participant Other as Other Members' WS

    Client->>WS: SEND_CHAT {conversation_id, body, message_type, client_msg_id}
    WS->>Service: SendChat()
    Service->>DB: InsertOrGet message (idempotent)
    Service->>Redis: PUBLISH conv:{id}:messages (if enabled)
    Service-->>WS: message stored
    WS-->>Mobile: RECEIVE_CHAT {message_id,...}
    WS-->>Other: RECEIVE_CHAT {message_id,...} (local fan-out if Redis not used)
    Note over Client: Clients cache recent messages per conversation and de-dupe by client_msg_id and fallback to GET history on reconnect.

    Note over Client,WS: Live location (optional)
    Client->>WS: PUBLISH_LIVE_LOCATION {stream_id, conversation_id, lat/lng,...}
    WS->>Service: PublishLocation()
    Service->>DB: Validate session + save latest point
    Service->>Redis: PUBLISH live:{conversation_id} (if enabled)
    WS-->>Mobile: SUBSCRIBE_LIVE_LOCATION {point}
    WS-->>Other: SUBSCRIBE_LIVE_LOCATION {point} (local fan-out if Redis not used)
    Note over Client: Clients cache last point per stream_id and evict on TTL expiry or stream end.
```
