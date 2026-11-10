import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Button,
  Card,
  Descriptions,
  Input,
  Progress,
  Segmented,
  Space,
  Spin,
  Table,
  Tag,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { getLot, type Lot, type LotStatus } from '@/api/lots'
import { listBobbins, BOBBIN_STATUS_LABEL, type Bobbin, type BobbinStatus } from '@/api/bobbins'
import { listGrades, qualifiedLabel, type Grade, type QualifiedFilter } from '@/api/grades'
import { listDoffings, DOFFING_STATUS_LABEL, type Doffing } from '@/api/doffings'

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

  // 丝锭列表：服务端分页 + 前端编码搜索（spec §3.2 明示搜索走客户端）
  const [bobbins, setBobbins] = useState<Bobbin[]>([])
  const [bobbinTotal, setBobbinTotal] = useState(0)
  const [bobbinPage, setBobbinPage] = useState(1)
  const [bobbinPageSize, setBobbinPageSize] = useState(20)
  const [bobbinLoading, setBobbinsLoading] = useState(false)
  const [keyword, setKeyword] = useState('')

  // 质检记录：按批次过滤，三态筛选
  const [grades, setGrades] = useState<Grade[]>([])
  const [gradeTotal, setGradeTotal] = useState(0)
  const [gradePage, setGradePage] = useState(1)
  const [gradePageSize, setGradePageSize] = useState(20)
  const [gradeLoading, setGradeLoading] = useState(false)
  const [qualifiedFilter, setQualifiedFilter] = useState<QualifiedFilter>('')

  // 落纱记录：用于追溯链的机台与操作员层
  const [doffings, setDoffings] = useState<Doffing[]>([])
  const [doffingTotal, setDoffingTotal] = useState(0)
  const [doffingLoading, setDoffingLoading] = useState(false)

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

  const loadBobbins = (page: number, size: number) => {
    if (!id) return

    setBobbinsLoading(true)
    listBobbins({ lot_id: id, page, page_size: size })
      .then(({ list, total }) => {
        setBobbins(list)
        setBobbinTotal(total)
      })
      .catch((err) => message.error(err instanceof Error ? err.message : '加载丝锭失败'))
      .finally(() => setBobbinsLoading(false))
  }

  useEffect(() => {
    loadBobbins(bobbinPage, bobbinPageSize)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, bobbinPage, bobbinPageSize])

  const loadGrades = (page: number, size: number, filter: QualifiedFilter) => {
    if (!id) return

    setGradeLoading(true)
    listGrades({ lot_id: id, page, page_size: size, is_qualified: filter })
      .then(({ list, total }) => {
        setGrades(list)
        setGradeTotal(total)
      })
      .catch((err) => message.error(err instanceof Error ? err.message : '加载质检记录失败'))
      .finally(() => setGradeLoading(false))
  }

  useEffect(() => {
    loadGrades(gradePage, gradePageSize, qualifiedFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, gradePage, gradePageSize, qualifiedFilter])

  // 落纱一次取前 50 条用于追溯链概览：单批次落纱数量有限，
  // 这里不做分页，避免为「概览」再引入一套分页状态。
  useEffect(() => {
    if (!id) return

    setDoffingLoading(true)
    listDoffings({ lot_id: id, page_size: 50 })
      .then(({ list, total }) => {
        setDoffings(list)
        setDoffingTotal(total)
      })
      .catch(() => {
        setDoffings([])
        setDoffingTotal(0)
      })
      .finally(() => setDoffingLoading(false))
  }, [id])

  // 编码搜索在客户端做：只过滤当前页，不重新请求。
  // 这依赖 spec §3.2 的前提「单批次丝锭数量有限」，故不引入服务端搜索参数。
  const filteredBobbins = keyword
    ? bobbins.filter((b) => b.bobbin_number.toLowerCase().includes(keyword.toLowerCase()))
    : bobbins

  const bobbinColumns: ColumnsType<Bobbin> = [
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
      // 后端透传库值，遇到未知枚举原样展示而不是崩溃或显示空白。
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

  // 丝锭 id → 编码，供质检记录与追溯链把 bobbin_id 显示成人能看懂的值。
  // 只覆盖当前页的丝锭；跨页的丝锭显示 UUID 前 8 位，不假装知道。
  const bobbinNumberByID = new Map(bobbins.map((b) => [b.id, b.bobbin_number]))
  const bobbinLabel = (id: string) => bobbinNumberByID.get(id) ?? id.slice(0, 8)

  const gradeColumns: ColumnsType<Grade> = [
    {
      title: '丝锭',
      dataIndex: 'bobbin_id',
      render: (v: string) => <span className="mono">{bobbinLabel(v)}</span>,
    },
    {
      title: '等级',
      dataIndex: 'grade',
      width: 80,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: '是否合格',
      dataIndex: 'is_qualified',
      width: 110,
      // 三态：true / false / null。null 是「未判定」，不是「不合格」。
      render: (v: boolean | null) => {
        const { text, color } = qualifiedLabel(v)
        return <Tag color={color}>{text}</Tag>
      },
    },
    {
      title: '综合得分',
      dataIndex: 'final_score',
      width: 100,
      align: 'right',
      render: (v: number | null) => (v ?? '—'),
    },
    {
      title: '缺陷原因',
      dataIndex: 'defect_reason',
      render: (v?: string) => v || '—',
    },
    {
      title: '检测员',
      dataIndex: 'inspector_id',
      width: 110,
      render: (v: string | null) =>
        v ? <span className="mono">{v.slice(0, 8)}</span> : <Tag>未记录</Tag>,
    },
    {
      title: '检测时间',
      dataIndex: 'inspected_at',
      width: 150,
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
    },
  ]

  // 追溯链上显示的各层计数。落纱是一次取前 50 条，故这里标注「最多 50」。
  const doffingByStatus = doffings.reduce<Record<string, number>>((acc, d) => {
    acc[d.status] = (acc[d.status] ?? 0) + 1
    return acc
  }, {})

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

      <Card
        type="inner"
        title="丝锭列表"
        style={{ marginTop: 16 }}
        extra={
          <Space>
            {/* 编码搜索走客户端过滤：单批次丝锭数量有限，服务端再做一个搜索参数属过度设计 */}
            <Input.Search
              allowClear
              placeholder="按丝锭编码筛选"
              style={{ width: 220 }}
              onChange={(e) => setKeyword(e.target.value)}
            />
            <Button
              icon={<ReloadOutlined />}
              onClick={() => loadBobbins(bobbinPage, bobbinPageSize)}
            >
              刷新
            </Button>
          </Space>
        }
      >
        <Table<Bobbin>
          rowKey="id"
          size="small"
          loading={bobbinLoading}
          columns={bobbinColumns}
          dataSource={filteredBobbins}
          pagination={{
            current: bobbinPage,
            pageSize: bobbinPageSize,
            total: bobbinTotal,
            showSizeChanger: true,
            pageSizeOptions: [20, 50, 100, 200],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, size) => {
              setBobbinPage(p)
              setBobbinPageSize(size)
            },
          }}
          locale={{ emptyText: '该批次暂无丝锭记录' }}
        />
      </Card>

      <Card
        type="inner"
        title="质检记录"
        style={{ marginTop: 16 }}
        extra={
          <Segmented<QualifiedFilter | 'all'>
            value={qualifiedFilter === '' ? 'all' : qualifiedFilter}
            onChange={(v) => {
              setGradePage(1)
              setQualifiedFilter(v === 'all' ? '' : v)
            }}
            options={[
              { label: '全部', value: 'all' },
              { label: '合格', value: 'true' },
              { label: '不合格', value: 'false' },
              { label: '未判定', value: 'unspecified' },
            ]}
          />
        }
      >
        <Table<Grade>
          rowKey="id"
          size="small"
          loading={gradeLoading}
          columns={gradeColumns}
          dataSource={grades}
          pagination={{
            current: gradePage,
            pageSize: gradePageSize,
            total: gradeTotal,
            showSizeChanger: true,
            pageSizeOptions: [20, 50, 100, 200],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, size) => {
              setGradePage(p)
              setGradePageSize(size)
            },
          }}
          locale={{ emptyText: '该批次暂无质检记录' }}
        />
      </Card>

      <Card type="inner" title="追溯链" style={{ marginTop: 16 }} loading={doffingLoading}>
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          <div>
            <Tag color="blue">批次</Tag>
            <span className="mono">{lot.lot_number}</span>
          </div>

          <div style={{ paddingLeft: 8, color: '#999' }}>↓</div>

          <div>
            <Tag color="geekblue">落纱</Tag>
            {doffings.length === 0 ? (
              <span>暂无落纱记录</span>
            ) : (
              <Space size={6} wrap>
                {Object.entries(doffingByStatus).map(([status, count]) => (
                  <Tag key={status}>
                    {DOFFING_STATUS_LABEL[status as keyof typeof DOFFING_STATUS_LABEL] ?? status} ×{' '}
                    {count}
                  </Tag>
                ))}
                <span style={{ color: '#999' }}>
                  （展示最近 {doffings.length} 条
                  {doffingTotal > doffings.length ? `，共 ${doffingTotal} 条` : ''}）
                </span>
              </Space>
            )}
          </div>

          <div style={{ paddingLeft: 8, color: '#999' }}>↓</div>

          <div>
            <Tag color="cyan">丝锭</Tag>
            <span>共 {bobbinTotal} 个</span>
          </div>

          <div style={{ paddingLeft: 8, color: '#999' }}>↓</div>

          <div>
            <Tag color="green">质检</Tag>
            <span>共 {gradeTotal} 条记录</span>
            {gradeTotal > 0 && (
              <span style={{ color: '#999' }}>
                （合格 {grades.filter((g) => g.is_qualified === true).length} / 不合格{' '}
                {grades.filter((g) => g.is_qualified === false).length} / 未判定{' '}
                {grades.filter((g) => g.is_qualified === null).length}
                {gradeTotal > grades.length ? '，仅统计当前页' : ''}）
              </span>
            )}
          </div>

          <div style={{ paddingLeft: 8, color: '#999' }}>↓</div>

          <div>
            <Tag>包装</Tag>
            <span style={{ color: '#999' }}>
              桶 / 托盘 / 纸箱 — Center 暂无这些表的数据，待 Edge 打包环节接入
            </span>
          </div>
        </Space>
      </Card>
    </Card>
  )
}
