const apiBase = (import.meta.env.VITE_API_BASE as string | undefined)?.replace(/\/$/, '') ?? ''
const wsBase = (import.meta.env.VITE_WS_URL as string | undefined)?.replace(/\/$/, '')

export const buildApiUrl = (path: string) => `${apiBase}${path}`

export const buildWsUrl = (username: string) => {
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

export const api = async <T,>(path: string, init?: RequestInit): Promise<T> => {
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

export const makeId = (prefix: string) =>
    `${prefix}-${Math.random().toString(36).slice(2, 8)}`

export const formatClock = (timestamp?: string | number) =>
    timestamp
        ? new Intl.DateTimeFormat('en', {
            hour: '2-digit',
            minute: '2-digit',
        }).format(new Date(timestamp))
        : ''

export const randomLocation = () => {
    const baseLat = 37.7749
    const baseLng = -122.4194
    const offset = () => (Math.random() - 0.5) * 0.02

    return {
        lat: Number((baseLat + offset()).toFixed(5)),
        lng: Number((baseLng + offset()).toFixed(5)),
    }
}
