# End-to-End Sequences

Sequence diagrams for the main app screens: login, conversation list, creating a new chat, and chatting with messages + live location.

## Login Screen
```mermaid
sequenceDiagram
    autonumber
    actor User
    participant API as HTTP API
    participant DB as Postgres

    User->>API: POST /v1/login {username}
    API->>DB: Upsert user by username
    DB-->>API: user_id, username
    API-->>User: 200 {user_id, username}
    User->>API: GET /ws?username=...
    API-->>User: event CONNECTED {user_id, username}
```

## Conversation Screen (List)
```mermaid
sequenceDiagram
    autonumber
    actor User
    participant API as HTTP API
    participant DB as Postgres

    User->>API: GET /v1/conversations?user_id=<uid>&type=all&limit=50
    API->>DB: SELECT conversations JOIN conversation_members WHERE user_id=<uid> ORDER BY created_at DESC
    DB-->>API: rows with member_ids
    API-->>User: 200 [{id,type,title,member_ids,...}]
```

## Create New Chat (from Conversation Screen)
```mermaid
sequenceDiagram
    autonumber
    actor User
    participant API as HTTP API
    participant DB as Postgres

    User->>API: POST /v1/conversations {type:"direct|group", title?, usernames:[...]}
    API->>DB: Upsert users
    API->>DB: Insert/find conversation (direct pair uniqueness enforced)
    API->>DB: Insert conversation_members (batch)
    DB-->>API: conversation + members
    API-->>User: 200 {id,type,title,direct_pair_key,member_ids,created_at}
```

## Chat Detail Screen (Send Chat + Live Location)
```mermaid
sequenceDiagram
    autonumber
    actor User
    participant WS as WebSocket
    participant Service as Messaging/LiveLocation Services
    participant DB as Postgres
    participant Redis as Redis Pub/Sub
    participant Other as Other Members' WS

    User->>WS: SEND_CHAT {conversation_id, body, message_type, client_msg_id}
    WS->>Service: SendChat()
    Service->>DB: InsertOrGet message (idempotent)
    Service->>Redis: PUBLISH conv:{id}:messages (if enabled)
    Service-->>WS: message stored
    WS-->>User: RECEIVE_CHAT {message_id,...}
    WS-->>Other: RECEIVE_CHAT {message_id,...} (local fan-out if Redis not used)

    Note over User,WS: Live location (optional)
    User->>WS: PUBLISH_LIVE_LOCATION {stream_id, conversation_id, lat/lng,...}
    WS->>Service: PublishLocation()
    Service->>DB: Validate session + save latest point
    Service->>Redis: PUBLISH live:{conversation_id} (if enabled)
    WS-->>User: SUBSCRIBE_LIVE_LOCATION {point}
    WS-->>Other: SUBSCRIBE_LIVE_LOCATION {point} (local fan-out if Redis not used)
```
