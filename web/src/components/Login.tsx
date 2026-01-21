import { createSignal } from 'solid-js'
import { api } from '../api'

type LoginProps = {
    onLogin: (session: { userId: string; username: string }) => void
}

export function Login(props: LoginProps) {
    const [username, setUsername] = createSignal('')
    const [loading, setLoading] = createSignal(false)
    const [error, setError] = createSignal('')

    const handleSubmit = async (e: Event) => {
        e.preventDefault()
        const name = username().trim()
        if (!name) return

        setLoading(true)
        setError('')
        try {
            const res = await api<{ user_id: string; username: string }>('/v1/login', {
                method: 'POST',
                body: JSON.stringify({ username: name }),
            })
            props.onLogin({ userId: res.user_id, username: res.username })
        } catch (err) {
            setError(String(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <div class="login-container">
            <div class="login-card">
                <h1>Welcome</h1>
                <p class="lede">Enter your username to start chatting</p>

                <form onSubmit={handleSubmit} class="stack">
                    <div class="field">
                        <label>Username</label>
                        <input
                            type="text"
                            value={username()}
                            onInput={(e) => setUsername(e.currentTarget.value)}
                            placeholder="e.g. Alice"
                            autofocus
                        />
                    </div>

                    <button type="submit" class="primary" disabled={loading() || !username()}>
                        {loading() ? 'Signing in...' : 'Continue'}
                    </button>

                    {error() && <p class="error-text">{error()}</p>}
                </form>
            </div>
        </div>
    )
}
