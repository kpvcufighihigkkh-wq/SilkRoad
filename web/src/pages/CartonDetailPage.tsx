import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Card, Descriptions, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  cartonStatusColor,
  cartonStatusLabel,
  formatCartonWeight,
  getCarton,
  type Carton,
} from '@/api/cartons'
import { listBobbins, BOBBIN_STATUS_LABEL, type Bobbin, type BobbinStatus } from '@/api/bobbins'
import { PageLoading } from '@/App'

function fmt(value: string | null): string {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'
}

const bobbinColumns: ColumnsType<Bobbin> = [
  {
    title: '丝锭编码',
    dataIndex: 'bobbin_number',
    render: (v: string) => <span className="mono">{v}</span>,
  },
  {
    title: '净重 (kg)',
    dataIndex: 'net_weight',
    align: 'right',
    width: 110,
    render: (v: number) => v.toLocaleString(undefined, { maximumFractionDigits: 2 }),
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 100,
    render: (v: string) => {
      const label = BOBBIN_STATUS_LABEL[v as BobbinStatus]
      return label ? <Tag>{label}</Tag> : <Tag>{v}</Tag>
    },
  },
]

/**
 * 纸箱详情（只读）。
 *
 * 箱内丝锭同托盘/桶：GET /v1/bobbins 不支持 carton_id 过滤，
 * 按批次取回后客户端过滤；纸箱的 lot_id 可空，为空时直接给空态。
 */
export default function CartonDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [carton, setCarton] = useState<Carton | null>(null)
  const [loading, setLoading] = useState(true)
  const [bobbins, setBobbins] = useState<Bobbin[]>([])
  const [bobbinsLoading, setBobbinsLoading] = useState(false)

  useEffect(() => {
    if (!id) return

    let cancelled = false
    setLoading(true)

    getCarton(id)
      .then((data) => {
        if (!cancelled) setCarton(data)
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载纸箱详情失败')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [id])

  useEffect(() => {
    if (!carton || !id) return
    if (!carton.lot_id) return // 未绑定批次，没有可查的范围

    let cancelled = false
    setBobbinsLoading(true)

    listBobbins({ lot_id: carton.lot_id, page: 1, page_size: 200 })
      .then(({ list }) => {
        if (!cancelled) setBobbins(list.filter((b) => b.carton_id === id))
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载丝锭失败')
      })
      .finally(() => {
        if (!cancelled) setBobbinsLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [carton, id])

  if (loading) return <PageLoading />
  if (!carton) {
    return (
      <Card>
        <Space direction="vertical">
          <span>纸箱不存在或已删除。</span>
          <Link to="/cartons">返回纸箱列表</Link>
        </Space>
      </Card>
    )
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        title={
          <Space>
            <span className="mono">{carton.carton_number}</span>
            <Tag color={cartonStatusColor(carton.status)}>{cartonStatusLabel(carton.status)}</Tag>
          </Space>
        }
        extra={<Link to="/cartons">返回列表</Link>}
      >
        <Descriptions column={3} size="middle">
          <Descriptions.Item label="所属批次">
            {carton.lot_id ? (
              <Link className="mono" to={`/lots/${carton.lot_id}`}>
                {carton.lot_id}
              </Link>
            ) : (
              '—'
            )}
          </Descriptions.Item>
          <Descriptions.Item label="所属托盘">
            {carton.pallet_id ? (
              <Link className="mono" to={`/pallets/${carton.pallet_id}`}>
                {carton.pallet_id.slice(0, 8)}
              </Link>
            ) : (
              '—'
            )}
          </Descriptions.Item>
          <Descriptions.Item label="装箱">
            {carton.bobbin_count}/{carton.capacity}
          </Descriptions.Item>

          <Descriptions.Item label="总重量">
            {formatCartonWeight(carton.total_weight)}
          </Descriptions.Item>
          <Descriptions.Item label="标签打印">
            {carton.label_printed ? `已打印 ${fmt(carton.printed_at)}` : '未打印'}
          </Descriptions.Item>
          <Descriptions.Item label="封箱时间">{fmt(carton.packed_at)}</Descriptions.Item>

          <Descriptions.Item label="创建时间">{fmt(carton.created_at)}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{fmt(carton.updated_at)}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        title="箱内丝锭"
        extra={
          <span style={{ color: 'rgba(0,0,0,0.45)' }}>
            实际 {bobbins.length} 枚 / 记数 {carton.bobbin_count} 枚
          </span>
        }
      >
        <Table<Bobbin>
          rowKey="id"
          columns={bobbinColumns}
          dataSource={bobbins}
          loading={bobbinsLoading}
          size="middle"
          pagination={false}
          locale={{ emptyText: '没有查到箱内丝锭（可能是丝锭尚未同步到 Center）' }}
        />
      </Card>
    </Space>
  )
}
