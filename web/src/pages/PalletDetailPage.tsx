import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Card, Descriptions, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  formatWeight,
  getPallet,
  palletStatusColor,
  palletStatusLabel,
  type Pallet,
} from '@/api/pallets'
import { listBobbins, BOBBIN_STATUS_LABEL, type Bobbin, type BobbinStatus } from '@/api/bobbins'
import { PageLoading } from '@/App'

function fmt(value: string | null): string {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'
}

const bobbinsColumns: ColumnsType<Bobbin> = [
  {
    title: '丝锭编码',
    dataIndex: 'bobbin_number',
    render: (v: string) => <span className="mono">{v}</span>,
  },
  { title: '位置', dataIndex: 'spinning_position', align: 'right', width: 80 },
  {
    title: '等级',
    dataIndex: 'grade',
    width: 80,
    render: (v?: string) => (v ? <Tag>{v}</Tag> : '—'),
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
  {
    title: '创建时间',
    dataIndex: 'created_at',
    width: 150,
    render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
  },
]

/**
 * 托盘详情。
 *
 * 只读，与批次详情同理：托盘由 Edge 包装线产生，Center 不提供写接口。
 *
 * 盘内丝锭的取法：`GET /v1/bobbins` 目前只支持 lot_id / status 过滤，
 * 没有 pallet_id 参数，因此按托盘所属批次取回后在客户端按 pallet_id 过滤。
 * 单批次丝锭数量有限（与批次详情页同一前提），不为此新增服务端参数。
 */
export default function PalletDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [pallet, setPallet] = useState<Pallet | null>(null)
  const [loading, setLoading] = useState(true)
  const [bobbins, setBobbins] = useState<Bobbin[]>([])
  const [bobbinsLoading, setBobbinsLoading] = useState(false)

  useEffect(() => {
    if (!id) return

    let cancelled = false
    setLoading(true)

    getPallet(id)
      .then((data) => {
        if (!cancelled) setPallet(data)
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载托盘详情失败')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [id])

  // 托盘拿到后才能按批次取丝锭：lot_id 来自托盘自身。
  useEffect(() => {
    if (!pallet || !id) return

    let cancelled = false
    setBobbinsLoading(true)

    listBobbins({ lot_id: pallet.lot_id, page: 1, page_size: 200 })
      .then(({ list }) => {
        if (!cancelled) setBobbins(list.filter((b) => b.pallet_id === id))
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
  }, [pallet, id])

  const columns = bobbinsColumns

  if (loading) return <PageLoading />
  if (!pallet) {
    return (
      <Card>
        <Space direction="vertical">
          <span>托盘不存在或已删除。</span>
          <Link to="/pallets">返回托盘列表</Link>
        </Space>
      </Card>
    )
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        title={
          <Space>
            <span className="mono">{pallet.pallet_code}</span>
            <Tag color={palletStatusColor(pallet.status)}>{palletStatusLabel(pallet.status)}</Tag>
          </Space>
        }
        extra={<Link to="/pallets">返回列表</Link>}
      >
        <Descriptions column={3} size="middle">
          <Descriptions.Item label="所属批次">
            <Link className="mono" to={`/lots/${pallet.lot_id}`}>
              {pallet.lot_id}
            </Link>
          </Descriptions.Item>
          <Descriptions.Item label="层级">{pallet.level}</Descriptions.Item>
          <Descriptions.Item label="丝锭数">{pallet.bobbins_count}</Descriptions.Item>

          <Descriptions.Item label="净重">{formatWeight(pallet.net_weight)}</Descriptions.Item>
          <Descriptions.Item label="毛重">{formatWeight(pallet.gross_weight)}</Descriptions.Item>
          <Descriptions.Item label="皮重">{formatWeight(pallet.tare_weight)}</Descriptions.Item>

          <Descriptions.Item label="来源设备">
            {pallet.edge_id ? (
              <span className="mono">{pallet.edge_id.slice(0, 8)}</span>
            ) : (
              '—'
            )}
          </Descriptions.Item>
          <Descriptions.Item label="标签打印">
            {pallet.printed ? `已打印 ${fmt(pallet.printed_at)}` : '未打印'}
          </Descriptions.Item>
          <Descriptions.Item label="封托时间">{fmt(pallet.completed_at)}</Descriptions.Item>

          <Descriptions.Item label="创建时间">{fmt(pallet.created_at)}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{fmt(pallet.updated_at)}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        title="盘内丝锭"
        extra={
          <span style={{ color: 'rgba(0,0,0,0.45)' }}>
            实际 {bobbins.length} 枚 / 记数 {pallet.bobbins_count} 枚
          </span>
        }
      >
        <Table<Bobbin>
          rowKey="id"
          columns={columns}
          dataSource={bobbins}
          loading={bobbinsLoading}
          size="middle"
          pagination={false}
          locale={{
            // 空表要说明原因，而不是只显示「暂无数据」：码垛中托盘本来就没有丝锭。
            emptyText:
              pallet.status === 'building'
                ? '该托盘尚未码入丝锭'
                : '没有查到盘内丝锭（可能是丝锭尚未同步到 Center）',
          }}
        />
      </Card>
    </Space>
  )
}
