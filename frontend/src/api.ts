export type Product = {
  id: number
  sku: string
  name: string
  price: number
  stock: number
  is_active: boolean
  created_at: string
  last_movement_at: string | null
}

export type ProductList = {
  data: Product[]
  page: number
  limit: number
  total: number
}

export type AdjustStockBody = {
  type: 'in' | 'out'
  qty: number
  note?: string
}

export type AdjustStockResult = {
  id: number
  stock: number
}

type ErrorBody = {
  code: string
  message: string
  sku?: string
  available?: number
  details?: string[]
}

export class ApiError extends Error {
  status: number
  code: string
  sku?: string
  available?: number
  details?: string[]

  constructor(status: number, body: ErrorBody) {
    super(body.message)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code
    this.sku = body.sku
    this.available = body.available
    this.details = body.details
  }
}


async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options?.headers },
  })

  if (!res.ok) {
    let body: ErrorBody = {
      code: 'HTTP_ERROR',
      message: `Request failed with status ${res.status}`,
    }
    try {
      const json = (await res.json()) as { error?: ErrorBody }
      if (json.error) body = json.error
    } catch {
      // body bukan JSON, pakai fallback di atas
    }
    throw new ApiError(res.status, body)
  }

  return (await res.json()) as T
}

export function listProducts(search: string): Promise<ProductList> {
  const params = new URLSearchParams({ search, limit: '100' })
  return request<ProductList>(`/api/products?${params}`)
}

export function adjustStock(id: number, body: AdjustStockBody): Promise<AdjustStockResult> {
  return request<AdjustStockResult>(`/api/products/${id}/stock`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}