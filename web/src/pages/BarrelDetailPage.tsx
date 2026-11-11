import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Card, Descriptions, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { barrelStatusColor, barrelStatusLabel, getBarrel, type Barrel } from '@/api/barrels'
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
    title: '锭位',
    // 桶内锭位是物理位置，与线体位号 spinning_position 不同。
    key: 'slot',
    width: 90,
    align: 'right',
    render: () => '—',
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
 * 桶详情（只读）。
 *
 * 盘内丝锭的取法同托盘详情：GET /v1/bobbins 不支持 barrel_id 过滤，
 * 按批次取回后在客户端按 barrel_id 过滤。单批次丝锭数量有限。
 */
export default function BarrelDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [barrel, setBarrel] = useState<Barrel | null>(null)
  const [loading, setLoading] = useState(true)
  const [bobbins, setBobbins] = useState<Bobbin[]>([])
  const [bobbinsLoading, setBobbinsLoading] = useState(false)

  useEffect(() => {
    if (!id) return

    let cancelled = false
    setLoading(true)

    getBarrel(id)
      .then((data) => {
        if (!cancelled) setBarrel(data)
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载桶详情失败')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [id])

  useEffect(() => {
    if (!barrel || !id) return

    let cancelled = false
    setBobbinsLoading(true)

    listBobbins({ lot_id: barrel.lot_id, page: 1, page_size: 200 })
      .then(({ list }) => {
        if (!cancelled) setBobbins(list.filter((b) => b.barrel_id === id))
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
  }, [barrel, id])

  if (loading) return <PageLoading />
  if (!barrel) {
    return (
      <Card>
        <Space direction="vertical">
          <span>桶不存在或已删除。</span>
          <Link to="/barrels">返回桶列表</Link>
        </Space>
      </Card>
    )
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        title={
          <Space>
            <span className="mono">{barrel.barrel_number}</span>
            <Tag color={barrelStatusColor(barrel.status)}>{barrelStatusLabel(barrel.status)}</Tag>
          </Space>
        }
        extra={<Link to="/barrels">返回列表</Link>}
      >
        <Descriptions column={3} size="middle">
          <Descriptions.Item label="所属批次">
            <Link className="mono" to={`/lots/${barrel.lot_id}`}>
              {barrel.lot_id}
            </Link>
          </Descriptions.Item>
          <Descriptions.Item label="装载">
            {barrel.current_count}/{barrel.capacity}
          </Descriptions.Item>
          <Descriptions.Item label="装满时间">{fmt(barrel.filled_at)}</Descriptions.Item>

          <Descriptions.Item label="创建时间">{fmt(barrel.created_at)}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{fmt(barrel.updated_at)}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        title="桶内丝锭"
        extra={
          <span style={{ color: 'rgba(0,0,0,0.45)' }}>
            实际 {bobbins.length} 枚 / 记数 {barrel.current_count} 枚
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
          locale={{ emptyText: '没有查到桶内丝锭（可能是丝锭尚未同步到 Center）' }}
        />
      </Card>
    </Space>
  )
}
