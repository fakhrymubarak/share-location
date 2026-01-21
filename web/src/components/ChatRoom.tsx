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
