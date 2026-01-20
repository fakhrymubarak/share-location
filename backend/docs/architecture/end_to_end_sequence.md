# End-to-End User Journey: Login to Live Location Sharing

The following sequence diagram illustrates the flow from a user logging in, connecting to the WebSocket, initializing a live location session, and broadcasting location updates to other users in a conversation.

```mermaid
sequenceDiagram
    autonumber
    actor Alice as Mobile App (Alice)
    actor Bob as Mobile App (Bob)
    participant API as Backend API (HTTP)
    participant WS as WebSocket Handler
    participant Service as LiveLocation Service
    participant DB as Postgres DB
    participant Redis as Redis Pub/Sub

    Note over Alice, Redis: 1. Authentication & Connection
    Alice->>API: POST /v1/login {username: "alice"}
    activate API
    API->>DB: Upsert User
    activate DB
    DB-->>API: User Record
    deactivate DB
    API-->>Alice: 200 OK {user_id, username}
    deactivate API

    Alice->>WS: Connect /ws?username=alice
    activate WS
    WS->>WS: Registry.Add(user_id, conn)
    WS-->>Alice: {"event": "CONNECTED", "data": {...}}

    Note over Bob: Bob connects similarly...
    Bob->>WS: Connect /ws?username=bob
    WS-->>Bob: {"event": "CONNECTED", ...}

    Note over Alice, Redis: 2. Start Live Location Session
    Alice->>API: POST /v1/live-locations {conversation_id, ttl_seconds: 900}
    activate API
    API->>Service: StartSession(user_id, conv_id, ttl)
    activate Service
    Service->>DB: Check Conversation Membership
    activate DB
    DB-->>Service: OK
    deactivate DB
    Service->>DB: Upsert Session (ExpiresAt = Now + TTL)
    activate DB
    DB-->>Service: Session Record
    deactivate DB
    Service-->>API: Session Data
    deactivate Service
    API-->>Alice: 200 OK {stream_id, expires_at}
    deactivate API

    Note over Alice, Redis: 3. Publish Live Location Updates
    loop Every N Seconds
        Alice->>WS: {"event": "PUBLISH_LIVE_LOCATION", "data": {...}}
        WS->>Service: PublishLocation(stream_id, lat, lng...)
        activate Service
        Service->>DB: Validate Session & Expiry
        Service->>DB: Save Point (Latest)
        
        par Fan-out
            Service->>Redis: Publish to Channel (Async)
            Service->>WS: Broadcast to Conversation Members
        end
        
        Service-->>WS: Point Saved
        deactivate Service
        
        WS-->>Alice: {"event": "SUBSCRIBE_LIVE_LOCATION", "data": {...}} (Ack)
        WS-->>Bob: {"event": "SUBSCRIBE_LIVE_LOCATION", "data": {...}}
    end

    Note over Bob: Bob updates UI with Alice's location
```
