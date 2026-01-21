import { For, Show, createMemo, createSignal, onCleanup } from 'solid-js'
import './App.css'

type SystemEvent = {
  id: string
  title: string
  detail: string
  tone?: 'info' | 'success' | 'muted' | 'warning'
}

type LocationPoint = {
  lat: number
  lng: number
  timestamp: number
}

type LiveLocation = {
  active: boolean
  streamId?: string
  expiresAt?: string
  points: LocationPoint[]
}

type Message = {
  id: string
  senderId: string
  senderName: string
  content: string
  timestamp: string
  type?: 'text' | 'location'
}

type Participant = {
  id: string
  username: string
}

type Chat = {
  id: string
  name: string
  isGroup: boolean
  participants: Participant[]
  messages: Message[]
  liveLocation: LiveLocation
}

type Screen = 'login' | 'conversations' | 'chat'

type Session = { userId: string; username: string }

const apiBase = (import.meta.env.VITE_API_BASE as string | undefined)?.replace(/\/$/, '') ?? ''
const wsBase = (import.meta.env.VITE_WS_URL as string | undefined)?.replace(/\/$/, '')

const makeId = (prefix: string) =>
  `${prefix}-${Math.random().toString(36).slice(2, 8)}`

const formatClock = (timestamp?: string | number) =>
  timestamp
    ? new Intl.DateTimeFormat('en', {
        hour: '2-digit',
        minute: '2-digit',
      }).format(new Date(timestamp))
    : ''

const randomLocation = () => {
  const baseLat = 37.7749
  const baseLng = -122.4194
  const offset = () => (Math.random() - 0.5) * 0.02

  return {
    lat: Number((baseLat + offset()).toFixed(5)),
    lng: Number((baseLng + offset()).toFixed(5)),
  }
}

const buildApiUrl = (path: string) => `${apiBase}${path}`

const buildWsUrl = (username: string) => {
  if (wsBase) {
    const hasQuery = wsBase.includes('?')
    return `${wsBase}${hasQuery ? '&' : '?'}username=${encodeURIComponent(username)}`
  }
  if (apiBase.startsWith('http')) {
    const wsRoot = apiBase.replace(/^http/, 'ws')
    return `${wsRoot}/ws?username=${encodeURIComponent(username)}`
  }
  const origin = window.location.origin.replace(/^http/, 'ws')
  return `${origin}/ws?username=${encodeURIComponent(username)}`
}

function App() {
  const [screen, setScreen] = createSignal<Screen>('login')
  const [session, setSession] = createSignal<Session | null>(null)
  const [wsStatus, setWsStatus] = createSignal<'idle' | 'connecting' | 'connected' | 'error'>('idle')
  const [systemEvents, setSystemEvents] = createSignal<SystemEvent[]>([
    {
      id: 'auth',
      title: 'Ready for login',
      detail: 'POST /v1/login will upsert your username and return identifiers.',
      tone: 'muted',
    },
    {
      id: 'socket',
      title: 'WebSocket idle',
      detail: 'Connect via /ws?username=<you> to broadcast updates.',
      tone: 'muted',
    },
  ])
  const [loginName, setLoginName] = createSignal('alice')
  const [chats, setChats] = createSignal<Chat[]>([])
  const [selectedChatId, setSelectedChatId] = createSignal<string | null>(null)
  const [newChatName, setNewChatName] = createSignal('Ops Bridge')
  const [newParticipants, setNewParticipants] = createSignal('alice, fakhry')
  const [newMember, setNewMember] = createSignal('')
  const [messageText, setMessageText] = createSignal('')
  const [featuresOpen, setFeaturesOpen] = createSignal(false)
  const [socket, setSocket] = createSignal<WebSocket | null>(null)

  const selectedChat = createMemo(
    () => chats().find((chat) => chat.id === selectedChatId()) || null
  )

  const appendEvent = (
    title: string,
    detail: string,
    tone: SystemEvent['tone'] = 'info'
  ) =>
    setSystemEvents((events) =>
      [{ id: makeId('ev'), title, detail, tone }, ...events].slice(0, 8)
    )

  const api = async <T,>(path: string, init?: RequestInit): Promise<T> => {
    const resp = await fetch(buildApiUrl(path), {
      headers: {
        'Content-Type': 'application/json',
        ...(init?.headers || {}),
      },
      ...init,
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(text || `Request failed (${resp.status})`)
    }
    return resp.json() as Promise<T>
  }

  const connectSocket = (username: string) => {
    socket()?.close()
    const ws = new WebSocket(buildWsUrl(username))
    setWsStatus('connecting')
    ws.onopen = () => {
      setWsStatus('connected')
      appendEvent('WebSocket CONNECTED', `Subscribed as ${username} on /ws`)
    }
    ws.onclose = () => {
      setWsStatus('idle')
      setSocket(null)
    }
    ws.onerror = () => {
      setWsStatus('error')
      appendEvent('WebSocket error', 'Check backend availability', 'warning')
    }
    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data)
        handleSocketEvent(payload)
      } catch (err) {
        console.error('WS parse', err)
      }
    }
    setSocket(ws)
  }

  onCleanup(() => socket()?.close())

  const addParticipant = (conversationId: string, participant: Participant) =>
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === conversationId &&
        !chat.participants.some((p) => p.id === participant.id)
          ? { ...chat, participants: [...chat.participants, participant] }
          : chat
      )
    )

  const ensureChat = (chatId: string, seed?: Partial<Chat>): Chat => {
    const existing = chats().find((c) => c.id === chatId)
    if (existing) return existing
    const created: Chat = {
      id: chatId,
      name: seed?.name || `Conversation ${chatId.slice(0, 6)}`,
      isGroup: seed?.isGroup ?? true,
      participants: seed?.participants || [],
      messages: [],
      liveLocation: seed?.liveLocation || { active: false, points: [] },
    }
    setChats((prev) => [...prev, created])
    return created
  }

  const addMessage = (conversationId: string, message: Message) =>
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === conversationId
          ? { ...chat, messages: [...chat.messages, message] }
          : chat
      )
    )

  const mapSenderName = (chat: Chat | undefined, senderId: string) => {
    if (!chat) return senderId.slice(0, 6)
    const match = chat.participants.find((p) => p.id === senderId)
    return match?.username || senderId.slice(0, 6)
  }

  const loadHistory = async (conversationId: string) => {
    const chat = chats().find((c) => c.id === conversationId)
    if (!chat || chat.messages.length > 0) return
    try {
      const history = await api<
        {
          id: string
          conversation_id: string
          sender_id: string
          message_type: string
          client_msg_id: string
          body: Record<string, any>
          created_at: string
        }[]
      >(`/v1/conversations/${conversationId}/messages?limit=50`)

      setChats((prev) =>
        prev.map((c) =>
          c.id === conversationId
            ? {
                ...c,
                messages: history.map((msg) => ({
                  id: msg.id,
                  senderId: msg.sender_id,
                  senderName: mapSenderName(c, msg.sender_id),
                  content:
                    typeof msg.body?.text === 'string'
                      ? msg.body.text
                      : JSON.stringify(msg.body || {}),
                  timestamp: msg.created_at,
                  type: msg.message_type === 'text' ? 'text' : 'location',
                })),
              }
            : c
        )
      )
    } catch (err) {
      appendEvent('Load messages failed', String(err), 'warning')
    }
  }

  const handleSocketEvent = (payload: any) => {
    switch (payload.event) {
      case 'CONNECTED': {
        const data = payload.data || {}
        if (data.user_id && data.username) {
          setSession({ userId: data.user_id, username: data.username })
        }
        break
      }
      case 'RECEIVE_CHAT': {
        const data = payload.data
        if (!data) return
        const convId = data.conversation_id
        const chat = ensureChat(convId)
        addParticipant(convId, { id: data.sender_id, username: mapSenderName(chat, data.sender_id) })
        addMessage(convId, {
          id: data.message_id || makeId('msg'),
          senderId: data.sender_id,
          senderName: mapSenderName(chat, data.sender_id),
          content:
            typeof data.body?.text === 'string'
              ? data.body.text
              : JSON.stringify(data.body || {}),
          timestamp: data.created_at,
          type: data.message_type === 'text' ? 'text' : 'location',
        })
        appendEvent('RECEIVE_CHAT', `Message on ${convId}`, 'info')
        break
      }
      case 'SUBSCRIBE_LIVE_LOCATION': {
        const data = payload.data
        if (!data) return
        const convId = data.conversation_id
        const chat = ensureChat(convId)
        const point = data.point || {}
        setChats((prev) =>
          prev.map((item) =>
            item.id === convId
              ? {
                  ...item,
                  liveLocation: {
                    ...item.liveLocation,
                    active: true,
                    streamId: data.stream_id,
                    points: [
                      ...item.liveLocation.points,
                      {
                        lat: point.lat,
                        lng: point.lng,
                        timestamp: Date.parse(data.updated_at) || Date.now(),
                      },
                    ],
                  },
                  messages: [
                    ...item.messages,
                    {
                      id: makeId('loc'),
                      senderId: data.sender_id || 'live',
                      senderName: mapSenderName(chat, data.sender_id || 'live'),
                      content: `Live at ${point.lat}, ${point.lng}`,
                      timestamp: data.updated_at,
                      type: 'location',
                    },
                  ],
                }
              : item
          )
        )
        appendEvent('SUBSCRIBE_LIVE_LOCATION', `Point fan-out for ${convId}`, 'success')
        break
      }
      case 'ERROR': {
        const data = payload.data || {}
        appendEvent('WS error', `${data.code || ''} ${data.message || ''}`, 'warning')
        break
      }
      default:
        break
    }
  }

  const handleLogin = async (event: Event) => {
    event.preventDefault()
    const username = loginName().trim()
    if (!username) return

    try {
      setWsStatus('connecting')
      appendEvent('POST /v1/login', `Authenticating ${username}`)
      const res = await api<{ user_id: string; username: string }>('/v1/login', {
        method: 'POST',
        body: JSON.stringify({ username }),
      })
      setSession({ userId: res.user_id, username: res.username })
      appendEvent('Login success', `User ${res.username} (${res.user_id})`, 'success')
      connectSocket(res.username)
      setScreen('conversations')
    } catch (err) {
      setWsStatus('error')
      appendEvent('Login failed', String(err), 'warning')
    }
  }

  const handleCreateChat = async (event: Event) => {
    event.preventDefault()
    const username = session()?.username
    if (!username) return

    const rawParticipants = newParticipants()
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean)
    if (!rawParticipants.includes(username)) {
      rawParticipants.unshift(username)
    }
    const unique = Array.from(new Set(rawParticipants))
    const type = unique.length > 2 ? 'group' : 'direct'
    const title =
      type === 'group'
        ? newChatName().trim() || `Group ${unique.join(', ')}`
        : ''
    try {
      const res = await api<{
        id: string
        type: string
        title: string
        user_ids: string[]
      }>('/v1/conversations', {
        method: 'POST',
        body: JSON.stringify({
          type,
          title,
          usernames: unique,
        }),
      })

      const participants: Participant[] = res.user_ids.map((id, idx) => ({
        id,
        username: unique[idx] || id.slice(0, 6),
      }))

      const chat: Chat = {
        id: res.id,
        name: res.title || (type === 'direct' ? `Direct: ${unique.join(', ')}` : title),
        isGroup: type === 'group',
        participants,
        messages: [],
        liveLocation: { active: false, points: [] },
      }

      setChats((prev) => {
        const existing = prev.find((c) => c.id === chat.id)
        if (existing) return prev
        return [chat, ...prev]
      })
      setSelectedChatId(chat.id)
      setScreen('chat')
      setNewChatName('')
      setNewParticipants('')
      appendEvent('Conversation created', `${chat.name}`, 'success')
      await loadHistory(chat.id)
    } catch (err) {
      appendEvent('Create conversation failed', String(err), 'warning')
    }
  }

  const addMemberToChat = () => {
    const member = newMember().trim()
    const chat = selectedChat()
    if (!member || !chat) return
    addParticipant(chat.id, { id: makeId('usr'), username: member })
    setNewMember('')
    appendEvent('Added participant (client-side)', `${member} joined ${chat.name}`, 'info')
  }

  const handleSendMessage = (event: Event) => {
    event.preventDefault()
    const chat = selectedChat()
    const sess = session()
    const ws = socket()
    const content = messageText().trim()
    if (!chat || !sess || !content || !ws || ws.readyState !== WebSocket.OPEN) return

    const reqId = makeId('req')
    const clientMsgId = makeId('client')
    const payload = {
      event: 'SEND_CHAT',
      req_id: reqId,
      data: {
        conversation_id: chat.id,
        body: { text: content },
        message_type: 'text',
        client_msg_id: clientMsgId,
      },
    }
    ws.send(JSON.stringify(payload))
    setMessageText('')
  }

  const startLiveLocation = async () => {
    const chat = selectedChat()
    const sess = session()
    if (!chat || !sess) return
    try {
      const res = await api<{
        stream_id: string
        expires_at: string
        conversation_id: string
        user_id: string
      }>('/v1/live-locations', {
        method: 'POST',
        body: JSON.stringify({
          user_id: sess.userId,
          conversation_id: chat.id,
          ttl_seconds: 900,
        }),
      })
      setChats((prev) =>
        prev.map((item) =>
          item.id === chat.id
            ? {
                ...item,
                liveLocation: {
                  active: true,
                  streamId: res.stream_id,
                  expiresAt: res.expires_at,
                  points: [],
                },
              }
            : item
        )
      )
      appendEvent('Live location started', `Stream ${res.stream_id} (TTL 15m)`, 'success')
      setFeaturesOpen(true)
    } catch (err) {
      appendEvent('Start live location failed', String(err), 'warning')
    }
  }

  const publishLocation = () => {
    const chat = selectedChat()
    const ws = socket()
    const sess = session()
    if (!chat || !chat.liveLocation.active || !ws || ws.readyState !== WebSocket.OPEN || !sess) {
      return
    }
    const point = randomLocation()
    const payload = {
      event: 'PUBLISH_LIVE_LOCATION',
      req_id: makeId('req'),
      data: {
        stream_id: chat.liveLocation.streamId,
        conversation_id: chat.id,
        lat: point.lat,
        lng: point.lng,
        accuracy: 3,
        speed: 0,
        heading: 0,
        expires_at: chat.liveLocation.expiresAt,
      },
    }
    ws.send(JSON.stringify(payload))
    appendEvent('Publish location', `Fan-out to ${chat.name}`, 'info')
  }

  const stopLiveLocation = () => {
    const chat = selectedChat()
    if (!chat) return
    setChats((prev) =>
      prev.map((item) =>
        item.id === chat.id
          ? { ...item, liveLocation: { ...item.liveLocation, active: false } }
          : item
      )
    )
    appendEvent('Live location closed', 'Session ended', 'warning')
  }

  const signedIn = () => Boolean(session())

  const stepClass = (target: Screen) =>
    `step ${screen() === target ? 'active' : ''} ${
      target === 'login' && wsStatus() === 'connected' ? 'done' : ''
    } ${screen() !== target && screen() !== 'login' && target === 'login' ? 'muted' : ''}`

  const screenIntro = {
    login: 'Authenticate before opening the socket',
    conversations: 'Browse and create conversations',
    chat: 'Chat room with live location fan-out',
  }

  return (
    <div class="page">
      <div class="backdrop" />

      <header class="masthead">
        <div>
          <p class="eyebrow">Login → WebSocket → Live location broadcast</p>
          <h1>
            Chat through the full backend journey
            <span class="accent"> with SolidJS</span>
          </h1>
          <p class="lede">
            Mirrors the sequence in backend/docs/architecture/end_to_end_sequence.md: authenticate,
            connect, start a live location session, and stream points to a conversation.
          </p>
          <div class="hero-pills">
            <span>POST /v1/login</span>
            <span>Connect /ws?username=you</span>
            <span>Broadcast PUBLISH_LIVE_LOCATION</span>
          </div>
        </div>
        <div class="stepper">
          <div class={stepClass('login')}>
            <span class="index">1</span>
            <div>
              <p class="step-title">Login</p>
              <small>POST /v1/login</small>
            </div>
          </div>
          <div class={`step ${screen() === 'conversations' ? 'active' : ''}`}>
            <span class="index">2</span>
            <div>
              <p class="step-title">List conversations</p>
              <small>Choose or create a room</small>
            </div>
          </div>
          <div class={`step ${screen() === 'chat' ? 'active' : ''}`}>
            <span class="index">3</span>
            <div>
              <p class="step-title">Chat & share</p>
              <small>Live location fan-out</small>
            </div>
          </div>
        </div>
      </header>

      <main class="screens">
        <Show when={screen() === 'login'}>
          <section class="pane login-pane">
            <div class="pane-header">
              <div>
                <p class="eyebrow">Screen: Login</p>
                <h2>Authenticate and open the socket</h2>
                <p class="lede">{screenIntro.login}</p>
              </div>
              <div class={`status-dot ${wsStatus()}`}>
                {wsStatus() === 'connected' ? '●' : '○'}
              </div>
            </div>
            <div class="split">
              <div class="panel glass">
                <div class="panel-head">
                  <span class="pill small">Step 1</span>
                  <span class="badge muted">/v1/login</span>
                </div>
                <form class="stack" onSubmit={handleLogin}>
                  <label class="field">
                    <span>Username</span>
                    <input
                      type="text"
                      value={loginName()}
                      onInput={(e) => setLoginName(e.currentTarget.value)}
                      placeholder="alice"
                    />
                  </label>
                  <button type="submit" class="primary">
                    {wsStatus() === 'connected' ? 'Reconnect' : 'Login & connect'}
                  </button>
                  <p class="hint">
                    Simulates authentication then connects to <code>/ws?username=&lt;you&gt;</code>.
                  </p>
                </form>
              </div>
              <div class="panel outline">
                <div class="panel-head">
                  <p class="eyebrow">Socket status</p>
                  <span class="badge muted">
                    {wsStatus() === 'connected'
                      ? `Connected as ${session()?.username ?? 'unknown'}`
                      : 'Not connected'}
                  </span>
                </div>
                <div class="status-copy">
                  <p class="status-line">
                    {wsStatus() === 'idle'
                      ? 'Waiting for login...'
                      : wsStatus() === 'connecting'
                      ? 'Connecting to WebSocket...'
                      : `Subscribed on /ws?username=${session()?.username ?? ''}`}
                  </p>
                  <div class="event-list compact">
                    <For each={systemEvents()}>
                      {(event) => (
                        <div class={`event ${event.tone || 'info'}`}>
                          <div class="event-dot" />
                          <div>
                            <p class="event-title">{event.title}</p>
                            <p class="event-detail">{event.detail}</p>
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </Show>

        <Show when={screen() === 'conversations'}>
          <section class="pane">
            <div class="pane-header">
              <div>
                <p class="eyebrow">Screen: Conversation list</p>
                <h2>Pick a room or create a new one</h2>
                <p class="lede">{screenIntro.conversations}</p>
              </div>
              <div class="pill small">Connected as {session()?.username || 'anon'}</div>
            </div>
            <div class="split">
              <div class="panel glass">
                <div class="panel-head">
                  <div>
                    <p class="eyebrow">Existing</p>
                    <h3>Conversations</h3>
                  </div>
                  <span class="badge">#{chats().length}</span>
                </div>
                <div class="chat-list">
                  <For each={chats()}>
                    {(chat) => (
                      <button
                        class={`chat-item ${selectedChatId() === chat.id ? 'active' : ''}`}
                        onClick={() => {
                          setSelectedChatId(chat.id)
                          setScreen('chat')
                          loadHistory(chat.id)
                        }}
                      >
                        <div>
                          <div class="chat-title">
                            <span>{chat.name}</span>
                            <span class="pill small">{chat.isGroup ? 'Group' : 'Direct'}</span>
                          </div>
                          <p class="chat-meta">
                            {chat.participants.map((p) => p.username).slice(0, 3).join(', ')}
                            {chat.participants.length > 3
                              ? ` +${chat.participants.length - 3}`
                              : ''}
                          </p>
                        </div>
                        <span class="chevron">→</span>
                      </button>
                    )}
                  </For>
                  <Show when={chats().length === 0}>
                    <p class="hint">No conversations yet. Create one below.</p>
                  </Show>
                </div>
              </div>

              <div class="panel outline">
                <div class="panel-head">
                  <div>
                    <p class="eyebrow">Add person / create chat group</p>
                    <h3>New conversation</h3>
                  </div>
                  <span class="badge muted">/v1/conversations</span>
                </div>
                <form class="stack" onSubmit={handleCreateChat}>
                  <label class="field">
                    <span>Chat name</span>
                    <input
                      type="text"
                      value={newChatName()}
                      onInput={(e) => setNewChatName(e.currentTarget.value)}
                      placeholder="Ops bridge"
                    />
                  </label>
                  <label class="field">
                    <span>Participants (comma separated)</span>
                    <input
                      type="text"
                      value={newParticipants()}
                      onInput={(e) => setNewParticipants(e.currentTarget.value)}
                      placeholder="alice, fakhry"
                    />
                  </label>
                  <button type="submit" class="primary" disabled={!signedIn()}>
                    Create & open chat
                  </button>
                  <p class="hint">You are added automatically.</p>
                </form>

                <div class="event-list">
                  <div class="panel-head">
                    <p class="eyebrow">Backend steps</p>
                    <h4>End-to-end sequence</h4>
                  </div>
                  <For each={systemEvents()}>
                    {(event) => (
                      <div class={`event ${event.tone || 'info'}`}>
                        <div class="event-dot" />
                        <div>
                          <p class="event-title">{event.title}</p>
                          <p class="event-detail">{event.detail}</p>
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </div>
            </div>
          </section>
        </Show>

        <Show when={screen() === 'chat'}>
          <section class="pane chat-pane">
            <div class="pane-header">
              <div>
                <p class="eyebrow">Screen: Chat room</p>
                <h2>{selectedChat()?.name || 'Pick a chat'}</h2>
                <p class="lede">{screenIntro.chat}</p>
              </div>
              <div class="top-actions">
                <button class="ghost" onClick={() => setScreen('conversations')}>
                  ← Conversations
                </button>
                <span class="pill small">
                  {wsStatus() === 'connected'
                    ? `Connected as ${session()?.username ?? ''}`
                    : 'Not connected'}
                </span>
              </div>
            </div>

            <Show
              when={selectedChat()}
              fallback={<div class="empty-state">Select a conversation to enter the room.</div>}
            >
              {(chat) => (
                <div class="chat-shell">
                  <div class="chat-meta-bar">
                    <div class="chips">
                      <For each={chat().participants}>
                        {(p) => <span class="pill">{p.username}</span>}
                      </For>
                    </div>
                    <div class="add-person">
                      <input
                        type="text"
                        value={newMember()}
                        onInput={(e) => setNewMember(e.currentTarget.value)}
                        placeholder="Add person (UI only)"
                      />
                      <button class="secondary" onClick={addMemberToChat}>
                        Add
                      </button>
                    </div>
                  </div>

                  <div class="messages">
                    <For each={chat().messages}>
                      {(message) => (
                        <div
                          class={`bubble ${message.senderId === session()?.userId ? 'self' : ''} ${
                            message.type === 'location' ? 'location' : ''
                          }`}
                        >
                          <div class="bubble-header">
                            <span class="sender">{message.senderName}</span>
                            <span class="time">{formatClock(message.timestamp)}</span>
                          </div>
                          <p>{message.content}</p>
                        </div>
                      )}
                    </For>
                  </div>

                  <form class="composer" onSubmit={handleSendMessage}>
                    <input
                      type="text"
                      value={messageText()}
                      onInput={(e) => setMessageText(e.currentTarget.value)}
                      placeholder={
                        signedIn()
                          ? 'Type a message like WhatsApp...'
                          : 'Login to start chatting'
                      }
                      disabled={!signedIn()}
                    />
                    <button type="submit" class="primary" disabled={!signedIn()}>
                      Send
                    </button>
                    <button
                      type="button"
                      class="secondary"
                      onClick={() => setFeaturesOpen((open) => !open)}
                    >
                      Additional features
                    </button>
                  </form>

                  <Show when={featuresOpen()}>
                    <div class="feature-drawer">
                      <div class="drawer-head">
                        <div>
                          <p class="eyebrow">Additional features</p>
                          <h4>Share live location</h4>
                          <p class="drawer-lede">
                            Matches Step 2 and 3 from the backend sequence: start a session then
                            stream coordinates to everyone in the conversation.
                          </p>
                        </div>
                        <button class="ghost" onClick={() => setFeaturesOpen(false)}>
                          Close
                        </button>
                      </div>

                      <div class="feature-card">
                        <div class="feature-status">
                          <span
                            class={`status-dot ${chat().liveLocation.active ? 'connected' : 'idle'}`}
                          >
                            ●
                          </span>
                          {chat().liveLocation.active ? (
                            <span>
                              Stream {chat().liveLocation.streamId} until{' '}
                              {chat().liveLocation.expiresAt
                                ? formatClock(chat().liveLocation.expiresAt)
                                : 'expiry pending'}
                            </span>
                          ) : (
                            <span>No live session yet</span>
                          )}
                        </div>

                        <div class="feature-actions">
                          <Show when={!chat().liveLocation.active}>
                            <button class="primary" onClick={startLiveLocation} disabled={!signedIn()}>
                              Start live location
                            </button>
                          </Show>
                          <Show when={chat().liveLocation.active}>
                            <div class="action-row">
                              <button
                                class="primary"
                                onClick={publishLocation}
                                disabled={!signedIn()}
                              >
                                Publish location
                              </button>
                              <button class="ghost" onClick={stopLiveLocation}>
                                Stop session
                              </button>
                            </div>
                          </Show>
                          <p class="hint">
                            Each publish sends <code>PUBLISH_LIVE_LOCATION</code> and broadcasts to
                            the room.
                          </p>
                        </div>

                        <div class="points">
                          <For each={chat().liveLocation.points}>
                            {(point) => (
                              <div class="point-row">
                                <span class="pill small">ACK</span>
                                <span>
                                  {point.lat}, {point.lng}
                                </span>
                                <span class="time">{formatClock(point.timestamp)}</span>
                              </div>
                            )}
                          </For>
                          <Show when={chat().liveLocation.points.length === 0}>
                            <p class="hint">No coordinates yet. Publish to see the feed.</p>
                          </Show>
                        </div>
                      </div>
                    </div>
                  </Show>
                </div>
              )}
            </Show>
          </section>
        </Show>
      </main>
    </div>
  )
}

export default App
