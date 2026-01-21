import { createSignal, createMemo, onCleanup, Show } from 'solid-js'
import './App.css'
import { Layout } from './components/Layout'
import { Login } from './components/Login'
import { ConversationList } from './components/ConversationList'
import { ChatRoom } from './components/ChatRoom'
import { buildWsUrl, makeId, api } from './api'
import type { Chat, Session, Message, Screen } from './types'

function App() {
  const [screen, setScreen] = createSignal<Screen>('login')
  const [session, setSession] = createSignal<Session | null>(null)
  const [chats, setChats] = createSignal<Chat[]>([])
  const [selectedChatId, setSelectedChatId] = createSignal<string | null>(null)
  const [socket, setSocket] = createSignal<WebSocket | null>(null)

  const selectedChat = createMemo(
    () => chats().find((chat) => chat.id === selectedChatId()) || null
  )

  const connectSocket = (username: string) => {
    socket()?.close()
    const ws = new WebSocket(buildWsUrl(username))

    ws.onclose = () => {
      setSocket(null)
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

  const handleSocketEvent = (payload: any) => {
    switch (payload.event) {
      case 'CONNECTED':
        // Session handled by login usually, but good to know
        break
      case 'RECEIVE_CHAT': {
        const data = payload.data
        if (!data) return
        const convId = data.conversation_id

        // Update or ensure chat exists (if backend supported fetching missing chat details on the fly)
        // For now we assume we might need to fetch it if we don't have it, but simplistically:
        const existing = chats().find(c => c.id === convId)

        const message: Message = {
          id: data.message_id || makeId('msg'),
          senderId: data.sender_id,
          senderName: existing?.participants.find(p => p.id === data.sender_id)?.username || data.sender_id,
          content: typeof data.body?.text === 'string' ? data.body.text : JSON.stringify(data.body || {}),
          timestamp: data.created_at,
          type: data.message_type === 'text' ? 'text' : 'location'
        }

        if (existing) {
          setChats(prev => prev.map(c =>
            c.id === convId ? { ...c, messages: [...c.messages, message] } : c
          ))
        } else {
          // New chat we don't know about? For now ignore or could fetch.
        }
        break
      }
      case 'SUBSCRIBE_LIVE_LOCATION': {
        const data = payload.data
        if (!data) return
        const convId = data.conversation_id
        const point = data.point || {}

        setChats(prev => prev.map(c => {
          if (c.id !== convId) return c

          // Add location message marker
          const locMsg: Message = {
            id: makeId('loc'),
            senderId: data.sender_id || 'live',
            senderName: c.participants.find(p => p.id === data.sender_id)?.username || 'Live',
            content: `Live at ${point.lat}, ${point.lng}`,
            timestamp: data.updated_at,
            type: 'location'
          }

          return {
            ...c,
            liveLocation: {
              ...c.liveLocation,
              active: true,
              streamId: data.stream_id,
              points: [...c.liveLocation.points, { lat: point.lat, lng: point.lng, timestamp: Date.parse(data.updated_at) }]
            },
            messages: [...c.messages, locMsg]
          }
        }))
        break
      }
    }
  }

  const handleLogin = async (userSession: Session) => {
    setSession(userSession)
    try {
      const convs = await api<{ id: string; type: string; title: string; created_at: string }[]>(`/v1/conversations?user_id=${userSession.userId}`, {
        headers: { 'X-Username': userSession.username } // Pass auth if needed, though mostly relying on path for this demo
      })
      // Map backend data to frontend Chat type (we need to populate participants manually as API returns minimal)
      // For this simple demo, we will map what we have.
      // Note: The backend list endpoint returns basic info.
      setChats(convs.map(c => ({
        id: c.id,
        name: c.title || 'Chat',
        isGroup: c.type === 'group',
        participants: [], // We don't have them from list endpoint yet. 
        // Improvement: We successfully loaded the list, but names might be missing.
        // Ideally we should fetch details or update backend to return them. 
        // For now, let's load them to fix the "empty list" bug.
        messages: [],
        liveLocation: { active: false, points: [] }
      })))
    } catch (e) { console.error('Failed to load conversations', e) }

    connectSocket(userSession.username)
    setScreen('conversations')
  }

  const handleLogout = () => {
    socket()?.close()
    setSocket(null)
    setSession(null)
    setChats([])
    setSelectedChatId(null)
    setScreen('login')
  }

  const handleSelectChat = async (chatId: string) => {
    setSelectedChatId(chatId)
    setScreen('chat')

    // Load history
    const chat = chats().find(c => c.id === chatId)
    if (chat && chat.messages.length === 0) {
      try {
        const history = await api<any[]>(`/v1/conversations/${chatId}/messages?limit=50`)
        setChats(prev => prev.map(c =>
          c.id === chatId ? {
            ...c,
            messages: history.map(msg => ({
              id: msg.id,
              senderId: msg.sender_id,
              senderName: c.participants.find(p => p.id === msg.sender_id)?.username || msg.sender_id,
              content: typeof msg.body?.text === 'string' ? msg.body.text : JSON.stringify(msg.body),
              timestamp: msg.created_at,
              type: msg.message_type === 'text' ? 'text' : 'location'
            }))
          } : c
        ))
      } catch (e) {
        console.error(e)
      }
    }
  }

  return (
    <Layout>
      <Show when={screen() === 'login'}>
        <Login onLogin={handleLogin} />
      </Show>

      <Show when={screen() === 'conversations'}>
        <ConversationList
          chats={chats()}
          onSelectChat={handleSelectChat}
          currentUserId={session()?.userId || ''}
          currentUsername={session()?.username || ''}
          onChatCreated={(newChat) => setChats(prev => [newChat, ...prev])}
          onLogout={handleLogout}
        />
      </Show>

      <Show when={screen() === 'chat' && selectedChat()}>
        <ChatRoom
          chat={selectedChat()!}
          session={session()!}
          socket={socket()}
          onBack={() => setScreen('conversations')}
          onUpdateChat={(id, updater) => setChats(prev => prev.map(c => c.id === id ? updater(c) : c))}
        />
      </Show>
    </Layout>
  )
}

export default App
