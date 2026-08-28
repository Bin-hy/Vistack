import { post } from '@/lib/http'

export interface RegisterPayload {
  username: string
  password: string
}

export interface RegisterResult {
  message: string
  user_id: string | number
}

export async function register(payload: RegisterPayload): Promise<RegisterResult> {
  return post<RegisterResult>('/auth/register', payload)
}
