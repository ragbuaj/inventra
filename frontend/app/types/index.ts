export interface Paginated<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface ListQuery {
  search?: string
  limit?: number
  offset?: number
  [key: string]: unknown
}

export interface AuthUser {
  id: string
  name: string
  email: string
  role_id: string
  role_name: string
}

export type BadgeColor = 'primary' | 'success' | 'warning' | 'error' | 'neutral' | 'info'
