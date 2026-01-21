# Share Location Web (SolidJS)

SolidJS UI that walks through the flow described in `backend/docs/architecture/end_to_end_sequence.md`: login, connect to the WebSocket, create/list chats, start a live location session, and broadcast location updates inside the room.

## Run locally

```bash
cd web
npm install
npm run dev
```

Then open `http://localhost:5173` in your browser.

The dev server proxies `/v1/*` and `/ws` to `http://localhost:8080`, which is the backend default. If your backend runs elsewhere, set env vars:

```bash
VITE_API_BASE=http://localhost:8081
VITE_WS_URL=ws://localhost:8081/ws
```

## What’s inside

- Login with a username using the real `POST /v1/login`, then auto-connects to `/ws?username=<you>`.
- Conversation list backed by `/v1/conversations` creation and WebSocket fan-out for incoming chats.
- WhatsApp-style chat room with message composer.
- “Additional features” drawer showing a live-location card: start a session (15m TTL), publish random coordinates, view fan-out acknowledgements for each point.

## Build

```bash
npm run build
```

Outputs production assets to `dist/`.
