import { useEffect, useState } from 'react'
import { ApiError, adjustStock, listProducts, type Product } from './api'

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

function timeAgo(iso: string | null): string {
  if (!iso) return '—'
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60) return 'just now'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hours = Math.floor(min / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

type Filter = 'all' | 'active' | 'inactive' | 'low'

export default function App() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [reloadKey, setReloadKey] = useState(0)
  const [adjusting, setAdjusting] = useState<Product | null>(null)

    useEffect(() => {
    let ignore = false

    const timer = setTimeout(() => {
      setLoading(true)
      setError(null)
      listProducts(search)
        .then((res) => {
          if (!ignore) setProducts(res.data)
        })
        .catch((err: unknown) => {
          if (!ignore) setError(err instanceof Error ? err.message : 'Unknown error')
        })
        .finally(() => {
          if (!ignore) setLoading(false)
        })
    }, 300)

    return () => {
      ignore = true
      clearTimeout(timer)
    }
  }, [search, reloadKey])

    const visible = products.filter((p) => {
    if (filter === 'active') return p.is_active
    if (filter === 'inactive') return !p.is_active
    if (filter === 'low') return p.stock <= 10
    return true
  })

  function reload() {
    setReloadKey((k) => k + 1)
  }
    return (
    <main>
      <h1>Products</h1>

      <div className="toolbar">
        <input
          placeholder="Search SKU or name..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select value={filter} onChange={(e) => setFilter(e.target.value as Filter)}>
          <option value="all">All</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
          <option value="low">Low stock (≤ 10)</option>
        </select>
      </div>
            {loading ? (
        <p>Loading...</p>
      ) : error ? (
        <div className="error">
          <p>Failed to load products: {error}</p>
          <button onClick={reload}>Retry</button>
        </div>
      ) : visible.length === 0 ? (
        <p>No products</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>SKU</th>
              <th>Name</th>
              <th>Price</th>
              <th>Stock</th>
              <th>Status</th>
              <th>Last movement</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {visible.map((p) => (
              <tr key={p.id}>
                <td>{p.sku}</td>
                <td>{p.name}</td>
                <td>{rupiah.format(p.price)}</td>
                <td>{p.stock}</td>
                <td>
                  <span className={p.is_active ? 'badge active' : 'badge inactive'}>
                    {p.is_active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td>{timeAgo(p.last_movement_at)}</td>
                <td>
                  <button onClick={() => setAdjusting(p)}>Adjust</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
            {adjusting && (
        <AdjustForm
          key={adjusting.id}
          product={adjusting}
          onCancel={() => setAdjusting(null)}
          onSaved={() => {
            setAdjusting(null)
            reload()
          }}
        />
      )}
    </main>
  )
}
type AdjustFormProps = {
  product: Product
  onCancel: () => void
  onSaved: () => void
}

function AdjustForm({ product, onCancel, onSaved }: AdjustFormProps) {
  const [type, setType] = useState<'in' | 'out'>('in')
  const [qty, setQty] = useState('')
  const [note, setNote] = useState('')
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

    async function save() {
    const n = Number(qty)
    if (!Number.isInteger(n) || n <= 0) {
      setFormError('Qty must be an integer greater than 0')
      return
    }

    setSaving(true)
    setFormError(null)
    try {
      await adjustStock(product.id, { type, qty: n, note: note.trim() || undefined })
      onSaved()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'INSUFFICIENT_STOCK') {
        setFormError(`Insufficient stock, available: ${err.available}`)
      } else {
        setFormError(err instanceof Error ? err.message : 'Unknown error')
      }
      setSaving(false)
    }
  }

    return (
    <form
      className="adjust"
      onSubmit={(e) => {
        e.preventDefault()
        void save()
      }}
    >
      <h2>
        Adjust stock: {product.name} ({product.sku}), current {product.stock}
      </h2>

      <select value={type} onChange={(e) => setType(e.target.value as 'in' | 'out')}>
        <option value="in">In</option>
        <option value="out">Out</option>
      </select>
      <input
        placeholder="Qty"
        inputMode="numeric"
        value={qty}
        onChange={(e) => setQty(e.target.value)}
      />
      <input
        placeholder="Note (optional)"
        value={note}
        onChange={(e) => setNote(e.target.value)}
      />

      <button type="submit" disabled={saving}>
        {saving ? 'Saving...' : 'Save'}
      </button>
      <button type="button" onClick={onCancel} disabled={saving}>
        Cancel
      </button>

      {formError && <p className="error">{formError}</p>}
    </form>
  )
}