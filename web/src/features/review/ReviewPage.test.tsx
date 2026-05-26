import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ReviewPage } from './ReviewPage'
import { getReviewQueue, decideReview, editReview } from '@/api/reviews'
import { listDatasets } from '@/api/tasks'
import type { ReviewQueueResponse } from '@/types'

// 只 mock 网络层，键盘流/编辑态走真实组件逻辑（E12）。
vi.mock('@/api/reviews', () => ({
  getReviewQueue: vi.fn(),
  decideReview: vi.fn(),
  editReview: vi.fn(),
}))
vi.mock('@/api/tasks', () => ({ listDatasets: vi.fn() }))

const queue: ReviewQueueResponse = {
  dataset_name: 'demo',
  pending_total: 2,
  form_schema: {
    version: 1,
    primary_cols: ['title'],
    columns: [
      { code: 'title', type: 'text', role: 'context', label: '标题' },
      {
        code: 'discipline',
        type: 'varchar',
        role: 'fill',
        label: '学科',
        field: { kind: 'single', options: [{ value: 'eng', label: '工程' }, { value: 'theory', label: '理论' }] },
      },
    ],
  },
  items: [
    { annotation_id: 101, task_id: 11, source_row_pk: '1', round: 1, annotator: 'anno', created_at: '2026-01-01', data: { fills: { discipline: 'eng' }, _source: 'human' }, source_row: { title: 'AAA' } },
    { annotation_id: 102, task_id: 12, source_row_pk: '2', round: 1, annotator: 'anno', created_at: '2026-01-01', data: { fills: { discipline: 'theory' }, _source: 'human' }, source_row: { title: 'BBB' } },
  ],
}

beforeEach(() => {
  vi.clearAllMocks()
  ;(listDatasets as Mock).mockResolvedValue([
    { id: 5, name: 'demo', status: 'READY', total_rows: 2, completed: 2, pending: 0, claimed: 0, form_schema_version: 1 },
  ])
  ;(getReviewQueue as Mock).mockResolvedValue(queue)
  ;(decideReview as Mock).mockResolvedValue(undefined)
  ;(editReview as Mock).mockResolvedValue(undefined)
})

function renderPage() {
  return render(
    <MemoryRouter>
      <ReviewPage />
    </MemoryRouter>,
  )
}

describe('ReviewPage 键盘流', () => {
  it('A 键裁决当前条为 approved', async () => {
    renderPage()
    await screen.findByRole('button', { name: /通过/ }) // 等队列就绪
    fireEvent.keyDown(document.body, { key: 'a' })
    await waitFor(() => expect(decideReview).toHaveBeenCalledWith(101, 'approved', undefined))
  })

  it('J 切到下一条后 A 裁决的是第二条', async () => {
    renderPage()
    await screen.findByRole('button', { name: /通过/ })
    expect(screen.getByText('AAA')).toBeInTheDocument() // 当前第 1 条源数据
    fireEvent.keyDown(document.body, { key: 'j' })
    await screen.findByText('BBB') // 推进到第 2 条
    fireEvent.keyDown(document.body, { key: 'a' })
    await waitFor(() => expect(decideReview).toHaveBeenCalledWith(102, 'approved', undefined))
  })

  it('E 进入编辑态，Esc 取消', async () => {
    renderPage()
    await screen.findByRole('button', { name: /通过/ })
    fireEvent.keyDown(document.body, { key: 'e' })
    await screen.findByText(/待补全/) // SchemaForm 出现
    fireEvent.keyDown(document.body, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByText(/待补全/)).toBeNull())
  })

  it('编辑态下 ⌘/Ctrl+Enter 调 editReview 改写并通过', async () => {
    renderPage()
    await screen.findByRole('button', { name: /通过/ })
    fireEvent.keyDown(document.body, { key: 'e' })
    await screen.findByText(/待补全/)
    fireEvent.keyDown(document.body, { key: 'Enter', ctrlKey: true })
    await waitFor(() => {
      expect(editReview).toHaveBeenCalledTimes(1)
      const [annId, data] = (editReview as Mock).mock.calls[0]
      expect(annId).toBe(101)
      expect(data).toMatchObject({ _source: 'reviewer-edited', fills: { discipline: 'eng' } })
    })
  })
})
