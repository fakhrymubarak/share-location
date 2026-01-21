export type SystemEvent = {
  id: string
  title: string
  detail: string
  tone?: 'info' | 'success' | 'muted' | 'warning'
}

export type LocationPoint = {
  lat: number
  lng: number
  timestamp: number
}

export type LiveLocation = {
  active: boolean
  streamId?: string
  expiresAt?: string
  points: LocationPoint[]
}

export type Message = {
  id: string
  senderId: string
  senderName: string
  content: string
  timestamp: string
  type?: 'text' | 'location'
  streamId?: string
  currentPoint?: LocationPoint
}

export type Participant = {
  id: string
  username: string
}

export type Chat = {
  id: string
  name: string
  isGroup: boolean
  participants: Participant[]
  messages: Message[]
  liveLocation: LiveLocation
}

export type Screen = 'login' | 'conversations' | 'chat'

export type Session = { userId: string; username: string }
