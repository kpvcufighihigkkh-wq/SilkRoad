import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Form, Input, Progress, Select, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  barrelStatusColor,
  barrelStatusLabel,
  listBarrels,
  type Barrel,
  type BarrelStatus,
} from '@/api/barrels'

const columns: ColumnsType<Barrel> = [
  {
    title: '桶号',
    dataIndex: 'barrel_number',
    render: (value: string, row) => (
      <Link className="mono" to={`/barrels/${row.id}`}>
        {value}
      </Link>
    ),
  },
  {
    title: '批次',
    dataIndex: 'lot_id',
    render: (value: string) => (
      <Link className="mono" to={`/lots/${value}`}>
        {value.slice(0, 8)}
      </Link>
    ),
  },
  {
    title: '装载',
    key: 'loading',
    width: 160,
    // 直接展示 N/容量 并配进度条：两个数字比「百分比」更能说明现场状态。
    render: (_, row) => (
      <Space size={8}>
        <Progress
          type="circle"
          size={28}
          percent={row.capacity ? Math.round((row.current_count / row.capacity) * 100) : 0}
        />
        <span>
          {row.current_count}/{row.capacity}
        </span>
      </Space>
    ),
  },
  {
    title: '状态',
    dataIndex: 'status',
    render: (value: string) => <Tag color={barrelStatusColor(value)}>{barrelStatusLabel(value)}</Tag>,
  },
  {
    title: '装满时间',
    dataIndex: 'filled_at',
    render: (value: string | null) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'),
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm'),
  },
]

interface FilterForm {
  status?: BarrelStatus
  lot_id?: string
}

export default function BarrelsPage() {
  const [form] = Form.useForm<FilterForm>()
  const [rows, setRows] = useState<Barrel[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (p: number, size: number, filters: FilterForm) => {
    setLoading(true)
    try {
      const { list, total } = await listBarrels({ page: p, page_size: size, ...filters })
      setRows(list)
      setTotal(total)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载桶失败')
    } finally {
      setLoading(false)
    }
  }, [])

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
              { value: 'active', label: '装载中' },
              { value: 'full', label: '已装满' },
              { value: 'sorted', label: '已分拣' },
              { value: 'packed', label: '已打包' },
            ]}
          />
        </Form.Item>

        <Form.Item name="lot_id" label="批次ID">
          <Input allowClear placeholder="完整 UUID" style={{ width: 300 }} />
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

      <Table<Barrel>
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
