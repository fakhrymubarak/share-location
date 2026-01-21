import { createSignal, For, onMount, createEffect } from 'solid-js'
import { api, formatClock, makeId, randomLocation } from '../api'
import type { Chat, Session } from '../types'

type ChatRoomProps = {
    chat: Chat
    session: Session
    socket: WebSocket | null
    onBack: () => void
    onUpdateChat: (chatId: string, updater: (c: Chat) => Chat) => void
}

export function ChatRoom(props: ChatRoomProps) {
    const [messageText, setMessageText] = createSignal('')
    let messagesEndRef: HTMLDivElement | undefined

    const scrollToBottom = () => {
        messagesEndRef?.scrollIntoView({ behavior: 'smooth' })
    }

    createEffect(() => {
        // Scroll on new messages
        if (props.chat.messages.length) {
            scrollToBottom()
        }
    })

    createEffect(async () => {
        const chatId = props.chat.id
        // If no messages, fetch history.
        // We check length <= 1 because sometimes we might have just the last_message from the list view.
        // But usually list view last_message is in correct order.
        // Let's stick to the logic: if we open it and want full history.
        // If we want to be smarter: check if we have "enough" messages or a specific flag.
        // For now, let's replicate App.tsx logic: if 0 messages (or maybe just 1 last_message which might be old), fetch.
        // Actually App.tsx logic was: if (chat.messages.length === 0).
        // note: The conversation list mapping puts "last_message" into messages array. So length might be 1.
        // But the user might want to see previous history too.
        // Let's fetch if we haven't fetched history for this session yet. 
        // A simple heuristic for this demo: fetch if length < 2.

        if (props.chat.messages.length === 0 || (props.chat.messages.length === 1 && props.chat.messages[0].id.startsWith('msg-'))) {
            // To avoid infinite re-fetching loops, we should ideally have a 'historyLoaded' flag.
            // But let's just fetch if length is 0 for safety, OR if we rely on the previous behavior.
            // The previous behavior was "if length === 0". 
            // BUT, the conversation list puts the last message in `messages`. So length is usually 1.
            // The App.tsx logic `chat.messages.length === 0` might have been flawed if LastMessage was present?
            // Let's re-read App.tsx:
            // setChats(convs.map(c => ... messages: c.last_message ? [...] : [] ...))
            // So if there was a last message, length was 1. App.tsx ONLY fetched if length == 0.
            // This means previously history was NOT fetched if there was a last message!
            // That seems like a bug in the old code that we might want to "fix" or at least preserve.
            // If the user wants history, we should fetch it.
        }

        // Let's implement robust fetching: always fetch history on mount/change if we suspect we don't have it.
        // Since we don't store "hasFetchedHistory", we'll just fetch if we have few messages.
        // A better way: fetch and merge.
        if (props.chat.messages.length < 2) {
            try {
                const history = await api<any[]>(`/v1/conversations/${chatId}/messages?limit=50`)
                if (history.length > 0) {
                    props.onUpdateChat(chatId, (c) => {
                        // Merge strategy: unique by ID
                        const existingIds = new Set(c.messages.map(m => m.id))
                        const newMessages = history.map(msg => ({
                            id: msg.id,
                            senderId: msg.sender_id,
                            senderName: c.participants.find(p => p.id === msg.sender_id)?.username || msg.sender_id,
                            content: typeof msg.body?.text === 'string' ? msg.body.text : JSON.stringify(msg.body),
                            timestamp: msg.created_at,
                            type: (msg.message_type === 'text' ? 'text' : 'location') as 'text' | 'location'
                        })).filter(m => !existingIds.has(m.id))

                        if (newMessages.length === 0) return c

                        // Sort just in case
                        const all = [...c.messages, ...newMessages].sort((a, b) =>
                            new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
                        )
                        return { ...c, messages: all }
                    })
                }
            } catch (err) {
                console.error("failed to fetch history", err)
            }
        }
    })

    onMount(() => {
        scrollToBottom()
    })

    const handleSendMessage = (e: Event) => {
        e.preventDefault()
        const text = messageText().trim()
        const ws = props.socket

        if (!text || !ws || ws.readyState !== WebSocket.OPEN) return

        const reqId = makeId('req')
        const clientMsgId = makeId('client')

        const payload = {
            event: 'SEND_CHAT',
            req_id: reqId,
            data: {
                conversation_id: props.chat.id,
                body: { text },
                message_type: 'text',
                client_msg_id: clientMsgId,
            },
        }
        ws.send(JSON.stringify(payload))
        setMessageText('')
    }

    const startLiveLocation = async () => {
        try {
            const res = await api<{
                stream_id: string
                expires_at: string
            }>('/v1/live-locations', {
                method: 'POST',
                body: JSON.stringify({
                    user_id: props.session.userId,
                    conversation_id: props.chat.id,
                    ttl_seconds: 900,
                }),
            })

            props.onUpdateChat(props.chat.id, (c) => ({
                ...c,
                liveLocation: {
                    active: true,
                    streamId: res.stream_id,
                    expiresAt: res.expires_at,
                    points: [],
                },
            }))
        } catch (err) {
            console.error('Failed to start live location', err)
        }
    }

    const publishLocation = () => {
        const ws = props.socket
        if (!props.chat.liveLocation.active || !ws || ws.readyState !== WebSocket.OPEN) return

        const point = randomLocation()
        const payload = {
            event: 'PUBLISH_LIVE_LOCATION',
            req_id: makeId('req'),
            data: {
                stream_id: props.chat.liveLocation.streamId,
                conversation_id: props.chat.id,
                lat: point.lat,
                lng: point.lng,
                accuracy: 3,
                speed: 0,
                heading: 0,
                expires_at: props.chat.liveLocation.expiresAt,
            },
        }
        ws.send(JSON.stringify(payload))
    }

    const stopLiveLocation = () => {
        props.onUpdateChat(props.chat.id, (c) => ({
            ...c,
            liveLocation: { ...c.liveLocation, active: false }
        }))
    }

    return (
        <div class="chat-room">
            <header class="chat-header">
                <button class="back-btn" onClick={props.onBack}>←</button>
                <div class="header-info">
                    <h3>{props.chat.name}</h3>
                    <span class="participants">{props.chat.participants.map(p => p.username).join(', ')}</span>
                </div>
                <div class="header-actions">
                    {!props.chat.liveLocation.active ? (
                        <button class="icon-btn" onClick={startLiveLocation} title="Share Live Location">📍</button>
                    ) : (
                        <div class="live-controls">
                            <span class="live-indicator">LIVE</span>
                            <button onClick={publishLocation} title="Update Location">📡</button>
                            <button onClick={stopLiveLocation} title="Stop Sharing">⏹</button>
                        </div>
                    )}
                </div>
            </header>

            <div class="messages-area">
                <For each={props.chat.messages}>
                    {(msg) => (
                        <div class={`message-row ${msg.senderId === props.session.userId ? 'own' : 'other'} ${msg.type === 'location' ? 'location-msg' : ''}`}>
                            <div class="message-bubble">
                                {msg.senderId !== props.session.userId && <div class="sender-name">{msg.senderName}</div>}
                                <div class="message-content">
                                    {msg.type === 'location' ? (
                                        <span>📍 {msg.content}</span>
                                    ) : (
                                        msg.content
                                    )}
                                </div>
                                <div class="message-time">{formatClock(msg.timestamp)}</div>
                            </div>
                        </div>
                    )}
                </For>
                <div ref={messagesEndRef} />
            </div>

            {props.chat.liveLocation.active && props.chat.liveLocation.points.length > 0 && (
                <div class="mini-map-overlay">
                    <div class="map-header">Latest Locations</div>
                    {/* Visualization of points could go here, for now just list last few */}
                    <div class="points-list">
                        <For each={props.chat.liveLocation.points.slice(-3)}>
                            {p => <div class="point">{p.lat.toFixed(4)}, {p.lng.toFixed(4)} <small>{formatClock(p.timestamp)}</small></div>}
                        </For>
                    </div>
                </div>
            )}

            <form class="message-input-area" onSubmit={handleSendMessage}>
                <input
                    type="text"
                    placeholder="Type a message..."
                    value={messageText()}
                    onInput={(e) => setMessageText(e.currentTarget.value)}
                />
                <button type="submit" disabled={!messageText()}>Send</button>
            </form>
        </div>
    )
}
