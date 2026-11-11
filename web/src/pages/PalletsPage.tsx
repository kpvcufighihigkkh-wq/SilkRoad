import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Form, Input, Select, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  formatWeight,
  listPallets,
  palletStatusColor,
  palletStatusLabel,
  type Pallet,
  type PalletStatus,
} from '@/api/pallets'

const columns: ColumnsType<Pallet> = [
  {
    title: '托盘号',
    dataIndex: 'pallet_code',
    render: (value: string, row) => (
      <Link className="mono" to={`/pallets/${row.id}`}>
        {value}
      </Link>
    ),
  },
  {
    title: '批次',
    dataIndex: 'lot_id',
    // 托盘只承载批次主键：批次详情页已有完整信息，这里做跳转而不是再查一次名称。
    render: (value: string) => (
      <Link className="mono" to={`/lots/${value}`}>
        {value.slice(0, 8)}
      </Link>
    ),
  },
  {
    title: '层级',
    dataIndex: 'level',
    width: 80,
    align: 'right',
  },
  {
    title: '丝锭数',
    dataIndex: 'bobbins_count',
    width: 90,
    align: 'right',
    render: (value: number) => value.toLocaleString(),
  },
  {
    title: '净重',
    dataIndex: 'net_weight',
    align: 'right',
    // 未称重是 null，不是 0：formatWeight 负责把 null 显示成「—」。
    render: (value: number | null) => formatWeight(value),
  },
  {
    title: '毛重',
    dataIndex: 'gross_weight',
    align: 'right',
    render: (value: number | null) => formatWeight(value),
  },
  {
    title: '状态',
    dataIndex: 'status',
    render: (value: string) => <Tag color={palletStatusColor(value)}>{palletStatusLabel(value)}</Tag>,
  },
  {
    title: '封托时间',
    dataIndex: 'completed_at',
    render: (value: string | null) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—'),
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm'),
  },
]

interface FilterForm {
  status?: PalletStatus
  lot_id?: string
}

export default function PalletsPage() {
  const [form] = Form.useForm<FilterForm>()
  const [rows, setRows] = useState<Pallet[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (p: number, size: number, filters: FilterForm) => {
      setLoading(true)
      try {
        const { list, total } = await listPallets({ page: p, page_size: size, ...filters })
        setRows(list)
        setTotal(total)
      } catch (err) {
        message.error(err instanceof Error ? err.message : '加载托盘失败')
      } finally {
        setLoading(false)
      }
    },
    [],
  )

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
              { value: 'building', label: '码垛中' },
              { value: 'completed', label: '已封托' },
              { value: 'shipped', label: '已发货' },
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

      <Table<Pallet>
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
