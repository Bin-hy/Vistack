import { post } from '@/lib/http'

export interface RegisterPayload {
  account: string
  password: string
}

export interface User {
  id: string | number
  account: string
  nickname?: string
  avatar?: string
}

export interface RegisterResult {
  user: User
}

export async function register(payload: RegisterPayload): Promise<RegisterResult> {
  return post<RegisterResult>('/auth/register', payload)
}