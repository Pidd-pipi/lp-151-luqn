import type { Identity } from '../types'

export function saveToken(token: string) {
  localStorage.setItem('gbtreehole_token', token)
}

export function getToken(): string | null {
  return localStorage.getItem('gbtreehole_token')
}

export function clearToken() {
  localStorage.removeItem('gbtreehole_token')
}

export function saveIdentity(identity: Identity) {
  localStorage.setItem('gbtreehole_identity', JSON.stringify(identity))
}

export function getIdentity(): Identity | null {
  const raw = localStorage.getItem('gbtreehole_identity')
  if (!raw) return null
  try {
    return JSON.parse(raw) as Identity
  } catch {
    return null
  }
}

export function saveIdentities(identities: Identity[]) {
  localStorage.setItem('gbtreehole_identities', JSON.stringify(identities))
}

export function getIdentities(): Identity[] {
  const raw = localStorage.getItem('gbtreehole_identities')
  if (!raw) return []
  try {
    return JSON.parse(raw) as Identity[]
  } catch {
    return []
  }
}
