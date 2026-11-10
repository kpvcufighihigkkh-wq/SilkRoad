import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button, Card, Descriptions, Progress, Space, Spin, Tag, message } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { getLot, type Lot, type LotStatus } from '@/api/lots'

function statusTag(status: LotStatus) {
  switch (status) {
    case 'in_progress':
      return <Tag color="processing">进行中</Tag>
    case 'paused':
      return <Tag color="warning">已暂停</Tag>
    case 'completed':
      return <Tag color="success">已完成</Tag>
    case 'cancelled':
      return <Tag>已取消</Tag>
  }
}

function fmt(value?: string): string {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'
}

/**
 * 批次详情。
 *
 * 本页只读：Center 不提供任何批次写接口（数据归属单向，Edge 是权威源），
 * 因此这里不出现编辑、删除或状态修正按钮 —— 那不是「暂未实现」，
 * 而是设计明确排除的能力。
 */
export default function LotDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [lot, setLot] = useState<Lot | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return

    let cancelled = false
    setLoading(true)

    getLot(id)
      .then((data) => {
        if (!cancelled) setLot(data)
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载批次详情失败')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    // 组件卸载或路由切换时丢弃过期响应，避免覆盖新页面的数据。
    return () => {
      cancelled = true
    }
  }, [id])

  if (loading) {
    return (
      <Card>
        <div style={{ display: 'grid', placeItems: 'center', minHeight: 240 }}>
          <Spin size="large" />
        </div>
      </Card>
    )
  }

  if (!lot) {
    return (
      <Card>
        <Space direction="vertical">
          <span>批次不存在或已被删除。</span>
          <Link to="/lots">
            <Button icon={<ArrowLeftOutlined />}>返回列表</Button>
          </Link>
        </Space>
      </Card>
    )
  }

  return (
    <Card
      title={
        <Space>
          <Link to="/lots">
            <Button type="text" icon={<ArrowLeftOutlined />} />
          </Link>
          <span className="mono">{lot.lot_number}</span>
          {statusTag(lot.status)}
        </Space>
      }
    >
      <Descriptions bordered column={2} size="middle">
        <Descriptions.Item label="PLC 批次号">{lot.plc_lot_number || '—'}</Descriptions.Item>
        <Descriptions.Item label="订单号">{lot.order_code || '—'}</Descriptions.Item>
        <Descriptions.Item label="产品类型">{lot.product_type}</Descriptions.Item>
        <Descriptions.Item label="产品规格">{lot.product_spec || '—'}</Descriptions.Item>
        <Descriptions.Item label="计划数量">{lot.planned_quantity.toLocaleString()}</Descriptions.Item>
        <Descriptions.Item label="实际数量">{lot.actual_quantity.toLocaleString()}</Descriptions.Item>
        <Descriptions.Item label="进度" span={2}>
          <Progress percent={Math.round(lot.progress)} />
        </Descriptions.Item>
        <Descriptions.Item label="边端设备">
          <span className="mono">{lot.edge_id || '—'}</span>
        </Descriptions.Item>
        <Descriptions.Item label="创建时间">{fmt(lot.created_at)}</Descriptions.Item>
        <Descriptions.Item label="开始时间">{fmt(lot.start_time)}</Descriptions.Item>
        <Descriptions.Item label="结束时间">{fmt(lot.end_time)}</Descriptions.Item>
      </Descriptions>
    </Card>
  )
}
