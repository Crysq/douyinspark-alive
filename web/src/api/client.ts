import axios, { AxiosError } from 'axios'

interface Envelope<T> { code: number; message: string; data?: T }

export const http = axios.create({ baseURL: '/api', timeout: 30000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('doyinspark.token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (response) => response,
  (error: AxiosError<Envelope<unknown>>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('doyinspark.token')
      if (!location.pathname.startsWith('/login')) location.href = '/login'
    }
    return Promise.reject(error)
  },
)

/** 统一拆包：后端固定返回 { code, message, data } */
export async function request<T>(config: Parameters<typeof http.request>[0]): Promise<T> {
  try {
    const response = await http.request<Envelope<T>>(config)
    if (response.data.code !== 0) throw new Error(response.data.message || '请求失败')
    return response.data.data as T
  } catch (error) {
    if (axios.isAxiosError(error)) {
      const body = error.response?.data as Envelope<unknown> | undefined
      const wrapped = new Error(body?.message || error.message || '网络异常') as Error & { payload?: unknown; status?: number }
      wrapped.payload = body?.data
      wrapped.status = error.response?.status
      throw wrapped
    }
    throw error
  }
}

export const get = <T>(url: string, params?: unknown) => request<T>({ url, method: 'GET', params })
export const post = <T>(url: string, data?: unknown) => request<T>({ url, method: 'POST', data })
export const patch = <T>(url: string, data?: unknown) => request<T>({ url, method: 'PATCH', data })
export const put = <T>(url: string, data?: unknown) => request<T>({ url, method: 'PUT', data })
export const del = <T>(url: string, params?: unknown) => request<T>({ url, method: 'DELETE', params })
