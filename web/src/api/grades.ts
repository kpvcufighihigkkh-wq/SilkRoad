import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { PaginatedResponse } from '@/types/api'

/**
 * 质检记录。与 service.GradeResponse 一一对应。
 *
 * is_qualified 是三态：true / false / null（质检员未陈述结论）。
 * 前端必须区分三种展示，不能把 null 显示成「不合格」。
 */
export interface Grade {
  id: string
  bobbin_id: string
  inspector_id: string | null
  grade: string
  final_score: number | null
  /** 五项等级类型分（sorting/weight/final/vision/knitting），未录入时为 null。 */
  scores: Record<string, number> | null
  /** 质检项判定（pass/fail/na），未录入时为 null。 */
  inspection_details: Record<string, string> | null
  is_qualified: boolean | null
  defect_reason?: string
  notes?: string
  inspected_at: string
  created_at: string
}

/** 三态筛选值。空串表示不过滤。 */
export type QualifiedFilter = '' | 'true' | 'false' | 'unspecified'

export interface GradeListQuery extends PageQuery {
  lot_id?: string
  bobbin_id?: string
  is_qualified?: QualifiedFilter
}

export async function listGrades(query: GradeListQuery): Promise<{ list: Grade[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.bobbin_id) params.bobbin_id = query.bobbin_id
  if (query.is_qualified) params.is_qualified = query.is_qualified

  const { data } = await http.get<PaginatedResponse<Grade>>('/v1/grades', { params })
  return toPageResult(data)
}

/** 三态的中文标签与颜色，供页面统一渲染。 */
export function qualifiedLabel(v: boolean | null): { text: string; color?: string } {
  if (v === true) return { text: '合格', color: 'success' }
  if (v === false) return { text: '不合格', color: 'error' }
  return { text: '未判定', color: 'default' }
}
