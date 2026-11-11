import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Form, Input, Select, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  cartonStatusColor,
  cartonStatusLabel,
  formatCartonWeight,
  listCartons,
  type Carton,
  type CartonStatus,
} from '@/api/cartons'

const columns: ColumnsType<Carton> = [
  {
    title: '箱号',
    dataIndex: 'carton_number',
    render: (value: string, row) => (
      <Link className="mono" to={`/cartons/${row.id}`}>
        {value}
      </Link>
    ),
  },
  {
    title: '批次',
    dataIndex: 'lot_id',
    // 纸箱的 lot_id 可空：箱子建出来时可能还没绑定批次。
    render: (value: string | null) =>
      value ? (
        <Link className="mono" to={`/lots/${value}`}>
          {value.slice(0, 8)}
        </Link>
      ) : (
        '—'
      ),
  },
  {
    title: '所属托盘',
    dataIndex: 'pallet_id',
    render: (value: string | null) =>
      value ? (
        <Link className="mono" to={`/pallets/${value}`}>
          {value.slice(0, 8)}
        </Link>
      ) : (
        '—'
      ),
  },
  {
    title: '装箱',
    key: 'loading',
    width: 110,
    align: 'right',
    render: (_, row) => `${row.bobbin_count}/${row.capacity}`,
  },
  {
    title: '总重量',
    dataIndex: 'total_weight',
    align: 'right',
    render: (value: number | null) => formatCartonWeight(value),
  },
  {
    title: '状态',
    dataIndex: 'status',
    render: (value: string) => <Tag color={cartonStatusColor(value)}>{cartonStatusLabel(value)}</Tag>,
  },
  {
    title: '封箱时间',
    dataIndex: 'packed_at',
    render: (value: string | null) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'),
  },
]

interface FilterForm {
  status?: CartonStatus
  lot_id?: string
  pallet_id?: string
}

export default function CartonsPage() {
  const [form] = Form.useForm<FilterForm>()
  const [rows, setRows] = useState<Carton[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (p: number, size: number, filters: FilterForm) => {
    setLoading(true)
    try {
      const { list, total } = await listCartons({ page: p, page_size: size, ...filters })
      setRows(list)
      setTotal(total)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载纸箱失败')
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
              { value: 'packing', label: '装箱中' },
              { value: 'packed', label: '已封箱' },
              { value: 'shipped', label: '已发货' },
            ]}
          />
        </Form.Item>

        <Form.Item name="lot_id" label="批次ID">
          <Input allowClear placeholder="完整 UUID" style={{ width: 300 }} />
        </Form.Item>

        <Form.Item name="pallet_id" label="托盘ID">
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

      <Table<Carton>
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
