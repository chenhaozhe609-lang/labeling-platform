import { api } from './client'
import type { ReviewQueueResponse, ReviewStatus } from '@/types'

// 随机抽检队列：某数据集下待审标注 + 源行 + form_schema（reviewer/admin）。
export async function getReviewQueue(datasetId: number, limit = 20): Promise<ReviewQueueResponse> {
  const { data } = await api.get<ReviewQueueResponse>('/reviews/queue', {
    params: { dataset_id: datasetId, limit },
  })
  return data
}

// 裁决一条标注：approved 通过 / needs_redo 打回重标。
export async function decideReview(
  annotationId: number,
  status: ReviewStatus,
  note?: string,
): Promise<void> {
  await api.post(`/reviews/${annotationId}/decision`, { status, note: note ?? '' })
}
