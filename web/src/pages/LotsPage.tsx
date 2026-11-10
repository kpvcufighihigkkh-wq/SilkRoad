import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Form, Input, Progress, Select, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { listLots, type Lot, type LotStatus, type ProductType } from '@/api/lots'

/**
 * 批次状态到 Tag 的映射。与 spec §3.4 一致。
 * 直接写死而不是查表生成：枚举改动时类型检查会先报错。
 */
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

const columns: ColumnsType<Lot> = [
  {
    title: '批次号',
    dataIndex: 'lot_number',
    render: (value: string, row) => (
      <Link className="mono" to={`/lots/${row.id}`}>
        {value}
      </Link>
    ),
  },
  {
    title: '产品',
    key: 'product',
    // 产品类型与规格拼接展示：只显示类型会让同类型不同规格的批次难以区分。
    render: (_, row) => (
      <span>
        {row.product_type}
        {row.product_spec ? ` ${row.product_spec}` : ''}
      </span>
    ),
  },
  {
    title: '订单号',
    dataIndex: 'order_code',
    render: (value?: string) => value || '—',
  },
  {
    title: '计划数量',
    dataIndex: 'planned_quantity',
    align: 'right',
    render: (value: number) => value.toLocaleString(),
  },
  {
    title: '实际数量',
    dataIndex: 'actual_quantity',
    align: 'right',
    render: (value: number) => value.toLocaleString(),
  },
  {
    title: '进度',
    dataIndex: 'progress',
    width: 140,
    // 直接使用后端下发的 progress，不在此重算 —— 后端已处理计划量为 0 的边界。
    render: (value: number) => <Progress percent={Math.round(value)} size="small" />,
  },
  {
    title: '状态',
    dataIndex: 'status',
    render: (value: LotStatus) => statusTag(value),
  },
  {
    title: '边端设备',
    dataIndex: 'edge_id',
    // 设备名映射需要 GET /v1/edges，而该端点仅管理员可调；
    // viewer 拿不到名称，因此统一展示 ID 前缀，避免角色间行为不一致。
    render: (value?: string) => (value ? <span className="mono">{value.slice(0, 8)}</span> : '—'),
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm'),
  },
]

interface FilterForm {
  status?: LotStatus
  product_type?: ProductType
  order_code?: string
  keyword?: string
}

export default function LotsPage() {
  const [form] = Form.useForm<FilterForm>()
  const [rows, setRows] = useState<Lot[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (p: number, size: number, filters: FilterForm) => {
      setLoading(true)
      try {
        const { list, total } = await listLots({ page: p, page_size: size, ...filters })
        setRows(list)
        setTotal(total)
      } catch (err) {
        message.error(err instanceof Error ? err.message : '加载批次失败')
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  // 首次进入与分页变化时重新拉取；筛选条件从表单读当前值，
  // 这样翻页不会丢掉已选筛选。
  useEffect(() => {
    void load(page, pageSize, form.getFieldsValue())
  }, [page, pageSize, load, form])

  const onSearch = (values: FilterForm) => {
    setPage(1)
    void load(1, pageSize, values)
  }

  const onReset = () => {
    form.resetFields()
    setPage(1)
    void load(1, pageSize, {})
  }

  return (
    <Card>
      <Form<FilterForm> form={form} layout="inline" onFinish={onSearch} style={{ marginBottom: 16 }}>
        <Form.Item name="status" label="状态">
          <Select
            allowClear
            placeholder="全部"
            style={{ width: 130 }}
            options={[
              { value: 'in_progress', label: '进行中' },
              { value: 'paused', label: '已暂停' },
              { value: 'completed', label: '已完成' },
              { value: 'cancelled', label: '已取消' },
            ]}
          />
        </Form.Item>

        <Form.Item name="product_type" label="产品类型">
          <Select
            allowClear
            placeholder="全部"
            style={{ width: 120 }}
            options={[
              { value: 'FDY', label: 'FDY' },
              { value: 'POY', label: 'POY' },
              { value: 'DTY', label: 'DTY' },
            ]}
          />
        </Form.Item>

        <Form.Item name="order_code" label="订单号">
          <Input allowClear placeholder="精确匹配" style={{ width: 150 }} />
        </Form.Item>

        <Form.Item name="keyword" label="批次号">
          <Input allowClear placeholder="模糊匹配" style={{ width: 160 }} />
        </Form.Item>

        <Form.Item>
          <Space>
            <Button type="primary" htmlType="submit">
              查询
            </Button>
            <Button onClick={onReset}>重置</Button>
          </Space>
        </Form.Item>
      </Form>

      <Table<Lot>
        rowKey="id"
        columns={columns}
        dataSource={rows}
        loading={loading}
        size="middle"
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: [20, 50, 100, 200],
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, size) => {
            setPage(p)
            setPageSize(size)
          },
        }}
      />
    </Card>
  )
}
