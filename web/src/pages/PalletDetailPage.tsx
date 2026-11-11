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
  palletTypeLabel,
  type Pallet,
} from '@/api/pallets'
import { listBobbins, BOBBIN_STATUS_LABEL, type Bobbin, type BobbinStatus } from '@/api/bobbins'
import {
  cartonStatusColor,
  cartonStatusLabel,
  formatCartonWeight,
  listCartons,
  type Carton,
} from '@/api/cartons'
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

const cartonColumns: ColumnsType<Carton> = [
  {
    title: '箱号',
    dataIndex: 'carton_number',
    render: (v: string, row) => (
      <Link className="mono" to={`/cartons/${row.id}`}>
        {v}
      </Link>
    ),
  },
  {
    title: '装箱',
    key: 'loading',
    width: 100,
    align: 'right',
    render: (_, row) => `${row.bobbin_count}/${row.capacity}`,
  },
  {
    title: '总重量',
    dataIndex: 'total_weight',
    width: 110,
    align: 'right',
    render: (v: number | null) => formatCartonWeight(v),
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 100,
    render: (v: string) => <Tag color={cartonStatusColor(v)}>{cartonStatusLabel(v)}</Tag>,
  },
  {
    title: '封箱时间',
    dataIndex: 'packed_at',
    width: 150,
    render: (v: string | null) => fmt(v),
  },
]

/**
 * 托盘详情。
 *
 * 只读，与批次详情同理：托盘由 Edge 包装线产生，Center 不提供写接口。
 *
 * 两种托盘类型装的东西不同：
 *   - bobbin 托盘（POY/FDY）装丝锭。GET /v1/bobbins 不支持 pallet_id 过滤，
 *     按批次取回后客户端按 pallet_id 过滤（单批次数量有限）。
 *   - carton 托盘（DTY）装纸箱。GET /v1/cartons 支持 pallet_id，直接筛。
 */
export default function PalletDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [pallet, setPallet] = useState<Pallet | null>(null)
  const [loading, setLoading] = useState(true)
  const [bobbins, setBobbins] = useState<Bobbin[]>([])
  const [bobbinsLoading, setBobbinsLoading] = useState(false)
  const [cartons, setCartons] = useState<Carton[]>([])
  const [cartonsLoading, setCartonsLoading] = useState(false)

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
    if (pallet.pallet_type === 'carton') return // DTY 托盘装纸箱，不查丝锭

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

  // DTY 托盘的盘内纸箱：cartons 接口支持 pallet_id，不必客户端过滤。
  useEffect(() => {
    if (!pallet || !id) return
    if (pallet.pallet_type !== 'carton') return

    let cancelled = false
    setCartonsLoading(true)

    listCartons({ pallet_id: id, page: 1, page_size: 200 })
      .then(({ list }) => {
        if (!cancelled) setCartons(list)
      })
      .catch((err) => {
        if (!cancelled) message.error(err instanceof Error ? err.message : '加载纸箱失败')
      })
      .finally(() => {
        if (!cancelled) setCartonsLoading(false)
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
            <Tag>{palletTypeLabel(pallet.pallet_type)}</Tag>
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
          <Descriptions.Item label={pallet.pallet_type === 'carton' ? '纸箱数' : '丝锭数'}>
            {pallet.pallet_type === 'carton' ? pallet.cartons_count : pallet.bobbins_count}
          </Descriptions.Item>

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

      {pallet.pallet_type === 'carton' ? (
        <Card
          title="盘内纸箱"
          extra={
            <span style={{ color: 'rgba(0,0,0,0.45)' }}>
              实际 {cartons.length} 箱 / 记数 {pallet.cartons_count} 箱
            </span>
          }
        >
          <Table<Carton>
            rowKey="id"
            columns={cartonColumns}
            dataSource={cartons}
            loading={cartonsLoading}
            size="middle"
            pagination={false}
            locale={{
              emptyText:
                pallet.status === 'building'
                  ? '该托盘尚未码入纸箱'
                  : '没有查到盘内纸箱（可能是纸箱尚未同步到 Center）',
            }}
          />
        </Card>
      ) : (
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
      )}
    </Space>
  )
}
