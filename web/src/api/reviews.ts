import { api } from './client'
import type { AnnotationData, ReviewQueueResponse, ReviewStatus } from '@/types'

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

// 改写并通过（B4.4）：reviewer 微调 fills 后直接通过，原标注被修正版取代。
export async function editReview(
  annotationId: number,
  data: AnnotationData,
  note?: string,
): Promise<void> {
  await api.post(`/reviews/${annotationId}/edit`, { data, note: note ?? '' })
}
