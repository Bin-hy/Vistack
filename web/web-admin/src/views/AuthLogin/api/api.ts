import { post } from '@/lib/http'

export interface LoginPayload {
  username: string
  password: string
}

export interface User {
  id: string | number
  username: string
  nickname?: string
  avatar_url?: string
}

export interface LoginResult {
  token: string
  user: User
}

export async function login(payload: LoginPayload): Promise<LoginResult> {
  return post<LoginResult>('/auth/login', payload)
}
