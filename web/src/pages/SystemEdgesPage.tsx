import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Modal,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { KeyOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import {
  createEdge,
  issueEdgeToken,
  listEdges,
  EDGE_STATUS_LABEL,
  type CreateEdgeInput,
  type Edge,
  type EdgeStatus,
} from '@/api/edges'

const { Text } = Typography

const STATUS_COLOR: Record<EdgeStatus, string> = {
  online: 'success',
  offline: 'default',
  error: 'error',
  maintenance: 'warning',
}

/**
 * 设备凭证的有效期。后端默认 30 天，可由 EDGE_TOKEN_TTL 环境变量覆盖。
 * 前端无法读取服务端配置，因此这里展示的是**默认值**，并明确说明
 * 「以服务端配置为准」—— 不能假装知道实际 TTL。
 */
const DEFAULT_TOKEN_TTL = '30 天'

export default function SystemEdgesPage() {
  const [rows, setRows] = useState<Edge[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [statusFilter, setStatusFilter] = useState<EdgeStatus | undefined>()
  const [loading, setLoading] = useState(false)

  const [createOpen, setCreateOpen] = useState(false)
  const [detail, setDetail] = useState<Edge | null>(null)
  const [issued, setIssued] = useState<{ edge_code: string; token: string } | null>(null)
  const [form] = Form.useForm<CreateEdgeInput>()

  const load = useCallback(async (p: number, size: number, status?: EdgeStatus) => {
    setLoading(true)
    try {
      const res = await listEdges({ page: p, page_size: size, status })
      setRows(res.list)
      setTotal(res.total)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载设备失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(page, pageSize, statusFilter)
  }, [page, pageSize, statusFilter, load])

  const submitCreate = async () => {
    const values = await form.validateFields()
    try {
      await createEdge(values)
      message.success('设备已注册')
      setCreateOpen(false)
      form.resetFields()
      void load(page, pageSize, statusFilter)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '注册失败')
    }
  }

  /**
   * 签发凭证。二次确认是必需的：
   * 签出的 token 在有效期内可代表该设备上传数据，误签等于把数据入口给出去。
   */
  const confirmIssue = (e: Edge) => {
    Modal.confirm({
      title: `为设备 ${e.edge_code} 签发新的访问凭证？`,
      content: (
        <Space direction="vertical" size={4}>
          <Text>有效期约 {DEFAULT_TOKEN_TTL}（以服务端 EDGE_TOKEN_TTL 配置为准）。</Text>
          <Text type="warning">
            凭证一旦签出即视为有效，旧凭证在到期前不会被自动吊销。请只交给对应设备。
          </Text>
        </Space>
      ),
      okText: '确认签发',
      cancelText: '取消',
      onOk: async () => {
        try {
          const res = await issueEdgeToken(e.edge_code)
          setIssued(res)
        } catch (err) {
          message.error(err instanceof Error ? err.message : '签发失败')
          return
        }
        message.success('凭证已签发')
      },
    })
  }

  const columns: ColumnsType<Edge> = [
    {
      title: '设备编码',
      dataIndex: 'edge_code',
      render: (v: string) => <span className="mono">{v}</span>,
    },
    { title: '设备名称', dataIndex: 'edge_name' },
    {
      title: 'IP',
      dataIndex: 'ip_address',
      render: (v?: string) => (v ? <span className="mono">{v}</span> : '—'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      render: (v: EdgeStatus) => <Tag color={STATUS_COLOR[v]}>{EDGE_STATUS_LABEL[v]}</Tag>,
    },
    { title: '版本', dataIndex: 'version', render: (v?: string) => v || '—' },
    {
      title: '最后在线',
      dataIndex: 'last_seen',
      render: (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '—'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_, e) => (
        <Space size="small">
          <Button size="small" type="link" onClick={() => setDetail(e)}>
            详情
          </Button>
          <Button size="small" type="link" icon={<KeyOutlined />} onClick={() => confirmIssue(e)}>
            签发凭证
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space wrap style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          注册设备
        </Button>
        <Segmented<EdgeStatus | 'all'>
          value={statusFilter ?? 'all'}
          onChange={(v) => {
            setPage(1)
            setStatusFilter(v === 'all' ? undefined : v)
          }}
          options={[
            { label: '全部', value: 'all' },
            ...(Object.keys(EDGE_STATUS_LABEL) as EdgeStatus[]).map((s) => ({
              label: EDGE_STATUS_LABEL[s],
              value: s,
            })),
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => void load(page, pageSize, statusFilter)}>
          刷新
        </Button>
      </Space>

      <Table<Edge>
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
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, size) => {
            setPage(p)
            setPageSize(size)
          },
        }}
      />

      <Modal
        title="注册设备"
        open={createOpen}
        onOk={submitCreate}
        onCancel={() => {
          setCreateOpen(false)
          form.resetFields()
        }}
        okText="注册"
        cancelText="取消"
        destroyOnHidden
      >
        <Form<CreateEdgeInput> form={form} layout="vertical" autoComplete="off">
          <Form.Item
            name="edge_code"
            label="设备编码"
            rules={[{ required: true, max: 50, message: '请输入设备编码（≤50 字符）' }]}
          >
            {/* 编码即设备身份，签发凭证与上传鉴权都以它为准，创建后不可改 */}
            <Input placeholder="例如 edge-001" />
          </Form.Item>
          <Form.Item
            name="edge_name"
            label="设备名称"
            rules={[{ required: true, max: 100, message: '请输入设备名称（≤100 字符）' }]}
          >
            <Input placeholder="例如 一号边端" />
          </Form.Item>
          <Form.Item
            name="ip_address"
            label="IP 地址"
            rules={[{ required: true, max: 50, message: '请输入 IP 地址' }]}
            extra="上传鉴权会校验来源 IP，必须与设备实际出口 IP 一致。"
          >
            <Input placeholder="例如 192.168.1.101" />
          </Form.Item>
          <Form.Item name="version" label="版本">
            <Input placeholder="可选" />
          </Form.Item>
          <Form.Item name="notes" label="备注">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`设备详情：${detail?.edge_code ?? ''}`}
        open={!!detail}
        onCancel={() => setDetail(null)}
        footer={null}
        destroyOnHidden
      >
        {detail && (
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="设备编码">
              <span className="mono">{detail.edge_code}</span>
            </Descriptions.Item>
            <Descriptions.Item label="设备名称">{detail.edge_name}</Descriptions.Item>
            <Descriptions.Item label="IP 地址">{detail.ip_address || '—'}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={STATUS_COLOR[detail.status]}>{EDGE_STATUS_LABEL[detail.status]}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="版本">{detail.version || '—'}</Descriptions.Item>
            <Descriptions.Item label="最后在线">
              {detail.last_seen ? dayjs(detail.last_seen).format('YYYY-MM-DD HH:mm') : '—'}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {dayjs(detail.created_at).format('YYYY-MM-DD HH:mm')}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      <Modal
        title="设备凭证"
        open={!!issued}
        onCancel={() => setIssued(null)}
        footer={
          <Button type="primary" onClick={() => setIssued(null)}>
            我已保存
          </Button>
        }
        destroyOnHidden
      >
        {issued && (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Text type="warning">凭证只在此处显示一次，关闭后无法再次查看。请立即复制保存。</Text>
            <Input.TextArea
              readOnly
              value={issued.token}
              autoSize={{ minRows: 3, maxRows: 6 }}
              className="mono"
              onFocus={(e) => e.currentTarget.select()}
            />
            <Text type="secondary">
              设备编码 <span className="mono">{issued.edge_code}</span> · 有效期约 {DEFAULT_TOKEN_TTL}
              （以服务端配置为准）
            </Text>
          </Space>
        )}
      </Modal>
    </Card>
  )
}
