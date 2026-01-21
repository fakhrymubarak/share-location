import { createSignal, For, Show } from 'solid-js'
import { api } from '../api'
import type { Chat } from '../types'

type ConversationListProps = {
    chats: Chat[]
    onSelectChat: (chatId: string) => void
    currentUserId: string
    currentUsername: string
    onChatCreated: (chat: Chat) => void
    onLogout: () => void
}

export function ConversationList(props: ConversationListProps) {
    const [isCreating, setIsCreating] = createSignal(false)
    const [newChatName, setNewChatName] = createSignal('')
    const [participants, setParticipants] = createSignal('')
    const [loading, setLoading] = createSignal(false)

    const handleCreateChat = async (e: Event) => {
        e.preventDefault()
        setLoading(true)

        // Parse participants
        const participantList = participants()
            .split(',')
            .map(p => p.trim())
            .filter(p => p.length > 0)

        // Add current user if not present
        if (props.currentUsername && !participantList.includes(props.currentUsername)) {
            participantList.unshift(props.currentUsername)
        }

        const uniqueParticipants = [...new Set(participantList)]

        // Logic: If > 2 people (Self + 2 others), default to group.
        // If 2 people (Self + 1 other), direct.
        const type = uniqueParticipants.length > 2 ? 'group' : 'direct'

        const title = type === 'group' ? newChatName() || `Group Chat` : ''

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
                    usernames: uniqueParticipants,
                }),
            })

            const newChat: Chat = {
                id: res.id,
                name: res.title || (type === 'direct' ? uniqueParticipants.join(', ') : 'New Chat'),
                isGroup: type === 'group',
                participants: res.user_ids.map((id, i) => ({
                    id,
                    username: uniqueParticipants[i] || id
                })),
                messages: [],
                liveLocation: { active: false, points: [] },
            }

            props.onChatCreated(newChat)
            setIsCreating(false)
            setNewChatName('')
            setParticipants('')
        } catch (err) {
            alert('Failed to create chat: ' + err)
        } finally {
            setLoading(false)
        }
    }

    return (
        <div class="conversation-list-container">
            <div class="list-header">
                <h2>Chats</h2>
                <div class="header-actions">
                    <button class="icon-btn" onClick={() => setIsCreating(!isCreating())} title="New Chat">
                        {isCreating() ? '✕' : '+'}
                    </button>
                    <button class="icon-btn logout-btn" onClick={props.onLogout} title="Logout">
                        ⎋
                    </button>
                </div>
            </div>

            <Show when={isCreating()}>
                <form class="create-chat-form" onSubmit={handleCreateChat}>
                    <input
                        placeholder="Participants (comma separated)"
                        value={participants()}
                        onInput={(e) => setParticipants(e.currentTarget.value)}
                    />
                    <Show when={participants().includes(',')}>
                        <input
                            placeholder="Group Name (Optional)"
                            value={newChatName()}
                            onInput={(e) => setNewChatName(e.currentTarget.value)}
                        />
                    </Show>
                    <button type="submit" disabled={loading() || !participants()}>
                        Create
                    </button>
                </form>
            </Show>

            <div class="chat-items">
                <For each={props.chats} fallback={<div class="empty-list">No conversations yet</div>}>
                    {(chat) => (
                        <div class="chat-row" onClick={() => props.onSelectChat(chat.id)}>
                            <div class="chat-avatar">
                                {chat.name.charAt(0).toUpperCase()}
                            </div>
                            <div class="chat-info">
                                <div class="chat-name">{chat.name}</div>
                                <div class="chat-preview">
                                    {chat.messages.length > 0
                                        ? chat.messages[chat.messages.length - 1].content
                                        : 'No messages'}
                                </div>
                            </div>
                        </div>
                    )}
                </For>
            </div>
        </div>
    )
}
