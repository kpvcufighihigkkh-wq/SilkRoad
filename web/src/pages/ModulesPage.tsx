import { useCallback, useEffect, useState } from 'react'
import { Button, Card, Form, Select, Space, Table, Tag, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  listModules,
  moduleStatusColor,
  moduleStatusLabel,
  type Module,
  type ModuleStatus,
} from '@/api/modules'

const columns: ColumnsType<Module> = [
  {
    title: '吊车编号',
    dataIndex: 'module_number',
    render: (value: string) => <span className="mono">{value}</span>,
  },
  {
    title: '挂位 1',
    dataIndex: 'barrel1_id',
    // 挂位存的是桶的 UUID：Center 侧没有按 ID 查桶的便捷路由，
    // 这里只显示前 8 位，不假装知道桶号。
    render: (value: string | null) =>
      value ? <span className="mono">{value.slice(0, 8)}</span> : '—',
  },
  {
    title: '挂位 2',
    dataIndex: 'barrel2_id',
    render: (value: string | null) =>
      value ? <span className="mono">{value.slice(0, 8)}</span> : '—',
  },
  {
    title: '状态',
    dataIndex: 'status',
    render: (value: string) => <Tag color={moduleStatusColor(value)}>{moduleStatusLabel(value)}</Tag>,
  },
  {
    title: '当前位置',
    dataIndex: 'current_location',
    render: (value: string) => value || '—',
  },
  {
    title: 'RFID',
    dataIndex: 'rfid',
    render: (value: string) => (value ? <span className="mono">{value}</span> : '—'),
  },
  {
    title: '创建时间',
    dataIndex: 'created_at',
    render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm'),
  },
]

interface FilterForm {
  status?: ModuleStatus
}

export default function ModulesPage() {
  const [form] = Form.useForm<FilterForm>()
  const [rows, setRows] = useState<Module[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (p: number, size: number, filters: FilterForm) => {
    setLoading(true)
    try {
      const { list, total } = await listModules({ page: p, page_size: size, ...filters })
      setRows(list)
      setTotal(total)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载吊车失败')
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
            style={{ width: 140 }}
            options={[
              { value: 'loading', label: '装车中' },
              { value: 'transporting', label: '运输中' },
              { value: 'sorting', label: '分拣中' },
              { value: 'warehouse', label: '已入库' },
              { value: 'idle', label: '空闲' },
            ]}
          />
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

      <Table<Module>
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
