import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosError } from 'axios'

export interface HttpError extends Error {
  status?: number
  details?: unknown
}

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api'

let tokenCache: string | null = null

export function getToken(): string | null {
  if (tokenCache) return tokenCache
  tokenCache = localStorage.getItem('token')
  return tokenCache
}

export function setToken(token?: string | null) {
  tokenCache = token ?? null
  if (token) localStorage.setItem('token', token)
  else localStorage.removeItem('token')
}

function createHttp(): AxiosInstance {
  const instance = axios.create({
    baseURL: BASE_URL,
    timeout: 15000,
    withCredentials: false,
    headers: {
      'Accept': 'application/json',
      'Content-Type': 'application/json',
      'X-Requested-With': 'XMLHttpRequest',
    },
  })

  // Request interceptor
  instance.interceptors.request.use((config) => {
    const token = getToken()
    if (token && config.headers && !('Authorization' in config.headers)) {
      config.headers['Authorization'] = `Bearer ${token}`
    }
    return config
  })

  // Response interceptor（遵循 HTTP 状态码语义：成功直接返回 AxiosResponse，错误统一处理）
  instance.interceptors.response.use(
    (resp) => resp,
    (error: AxiosError) => {
      const err: HttpError = new Error('网络错误')
      if (error.response) {
        err.status = error.response.status
        const body: any = error.response.data
        err.message = (body?.message ?? body?.error ?? error.message ?? '请求失败') as string
        err.details = body
        if (error.response.status === 401) {
          setToken(null)
        }
      } else if (error.request) {
        err.message = '无法连接服务器，请检查网络或稍后重试'
      } else {
        err.message = error.message ?? '未知错误'
      }
      throw err
    },
  )

  return instance
}

export const http = createHttp()

// 便捷方法（返回 resp.data，保持与后端返回值一致）
export async function request<T = unknown>(config: AxiosRequestConfig): Promise<T> {
  const resp = await http.request(config)
  return resp.data as T
}

export async function get<T = unknown>(url: string, params?: unknown, config: AxiosRequestConfig = {}): Promise<T> {
  const resp = await http.get(url, { ...config, params })
  return resp.data as T
}

export async function post<T = unknown>(url: string, data?: unknown, config: AxiosRequestConfig = {}): Promise<T> {
  const resp = await http.post(url, data, config)
  return resp.data as T
}

export async function put<T = unknown>(url: string, data?: unknown, config: AxiosRequestConfig = {}): Promise<T> {
  const resp = await http.put(url, data, config)
  return resp.data as T
}

export async function del<T = unknown>(url: string, config: AxiosRequestConfig = {}): Promise<T> {
  const resp = await http.delete(url, config)
  return resp.data as T
}

// 可选：导出基础 URL，便于页面显示或调试
export const baseURL = BASE_URL