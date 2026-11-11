import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  Modal,
  Radio,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import {
  changePassword,
  createUser,
  resetPassword,
  deleteUser,
  listUsers,
  updateUser,
  ROLE_LABEL,
  ROLE_RANK,
  type CreateUserInput,
  type Role,
  type UpdateUserInput,
  type User,
} from '@/api/users'
import { getUser } from '@/auth/session'

const { Text } = Typography

const ROLE_COLOR: Record<Role, string> = {
  admin: 'red',
  inspector: 'blue',
  operator: 'green',
  viewer: 'default',
}

interface UserFormValues {
  username: string
  password?: string
  real_name: string
  role: Role
  email?: string
  phone?: string
}

export default function SystemUsersPage() {
  const me = getUser()
  const [rows, setRows] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [roleFilter, setRoleFilter] = useState<Role | undefined>()
  const [loading, setLoading] = useState(false)

  const [editing, setEditing] = useState<User | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [pwdTarget, setPwdTarget] = useState<User | null>(null)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [form] = Form.useForm<UserFormValues>()
  const [pwdForm] = Form.useForm<{ old_password: string; new_password: string }>()
  const [resetForm] = Form.useForm<{ new_password: string }>()

  const load = useCallback(
    async (p: number, size: number, role?: Role) => {
      setLoading(true)
      try {
        const res = await listUsers({ page: p, page_size: size, role })
        setRows(res.list)
        setTotal(res.total)
      } catch (err) {
        message.error(err instanceof Error ? err.message : '加载用户失败')
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  useEffect(() => {
    void load(page, pageSize, roleFilter)
  }, [page, pageSize, roleFilter, load])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ role: 'viewer' })
    setModalOpen(true)
  }

  const openEdit = (u: User) => {
    setEditing(u)
    form.setFieldsValue({
      username: u.username,
      real_name: u.real_name,
      role: u.role,
      email: u.email,
      phone: u.phone,
    })
    setModalOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()

    try {
      if (editing) {
        // 三态：字段留空表示「清空」（email/phone 由后端支持 &""），
        // role 与 is_active 不在此表单内，故不传。
        const input: UpdateUserInput = {
          real_name: values.real_name,
          email: values.email ?? '',
          phone: values.phone ?? '',
          role: values.role,
        }
        await updateUser(editing.id, input)
        message.success('已更新')
      } else {
        const input: CreateUserInput = {
          username: values.username,
          password: values.password ?? '',
          real_name: values.real_name,
          role: values.role,
          email: values.email || undefined,
          phone: values.phone || undefined,
        }
        await createUser(input)
        message.success('已创建')
      }
      setModalOpen(false)
      void load(page, pageSize, roleFilter)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存失败')
    }
  }

  const toggleActive = async (u: User) => {
    try {
      await updateUser(u.id, { is_active: !u.is_active })
      message.success(u.is_active ? '已停用' : '已启用')
      void load(page, pageSize, roleFilter)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '操作失败')
    }
  }

  const remove = (u: User) => {
    Modal.confirm({
      title: `删除用户 ${u.username}？`,
      content: '删除后该账号立即失效，且不可恢复。若只是暂时禁用人，建议改用「停用」。',
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteUser(u.id)
          message.success('已删除')
          void load(page, pageSize, roleFilter)
        } catch (err) {
          message.error(err instanceof Error ? err.message : '删除失败')
        }
      },
    })
  }

  const submitPassword = async () => {
    const values = await pwdForm.validateFields()
    if (!pwdTarget) return

    try {
      await changePassword(pwdTarget.id, values)
      message.success('密码已修改')
      setPwdTarget(null)
      pwdForm.resetFields()
    } catch (err) {
      message.error(err instanceof Error ? err.message : '修改密码失败')
    }
  }

  const submitReset = async () => {
    const values = await resetForm.validateFields()
    if (!resetTarget) return

    try {
      await resetPassword(resetTarget.id, values.new_password)
      message.success(`已重置 ${resetTarget.username} 的密码`)
      setResetTarget(null)
      resetForm.resetFields()
    } catch (err) {
      message.error(err instanceof Error ? err.message : '重置密码失败')
    }
  }

  // 只能分配不高于自己的角色：后端也会拦，这里提前收敛选项避免无效操作。
  const myRank = me ? ROLE_RANK[me.role] : 1
  const assignableRoles = (Object.keys(ROLE_RANK) as Role[]).filter((r) => ROLE_RANK[r] <= myRank)

  // 改密（要旧密码）只对自己显示：那是本人的操作。
  // 管理员另有一条「重置」路径（不要旧密码），用于员工忘密码与账号交接，
  // 因此对 admin 下所有用户都可用，包括自己。
  const canChangePassword = (u: User) => me?.id === u.id
  const canResetPassword = () => me?.role === 'admin'

  const columns: ColumnsType<User> = [
    { title: '用户名', dataIndex: 'username', render: (v: string) => <span className="mono">{v}</span> },
    { title: '姓名', dataIndex: 'real_name' },
    {
      title: '角色',
      dataIndex: 'role',
      render: (v: Role) => <Tag color={ROLE_COLOR[v]}>{ROLE_LABEL[v]}</Tag>,
    },
    { title: '邮箱', dataIndex: 'email', render: (v?: string) => v || '—' },
    { title: '电话', dataIndex: 'phone', render: (v?: string) => v || '—' },
    {
      title: '状态',
      dataIndex: 'is_active',
      render: (v: boolean) => (v ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 240,
      render: (_, u) => (
        <Space size="small">
          <Button size="small" type="link" onClick={() => openEdit(u)}>
            编辑
          </Button>
          {canChangePassword(u) && (
            <Button size="small" type="link" onClick={() => setPwdTarget(u)}>
              改密
            </Button>
          )}
          {canResetPassword() && (
            <Button size="small" type="link" danger onClick={() => setResetTarget(u)}>
              重置
            </Button>
          )}
          <Button size="small" type="link" onClick={() => void toggleActive(u)}>
            {u.is_active ? '停用' : '启用'}
          </Button>
          <Button
            size="small"
            type="link"
            danger
            disabled={me?.id === u.id}
            onClick={() => remove(u)}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space wrap style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新建用户
        </Button>
        <Segmented<Role | 'all'>
          value={roleFilter ?? 'all'}
          onChange={(v) => {
            setPage(1)
            setRoleFilter(v === 'all' ? undefined : v)
          }}
          options={[
            { label: '全部', value: 'all' },
            ...(Object.keys(ROLE_LABEL) as Role[]).map((r) => ({
              label: ROLE_LABEL[r],
              value: r,
            })),
          ]}
        />
        <Button icon={<ReloadOutlined />} onClick={() => void load(page, pageSize, roleFilter)}>
          刷新
        </Button>
      </Space>

      <Table<User>
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
        title={editing ? `编辑用户 ${editing.username}` : '新建用户'}
        open={modalOpen}
        onOk={submit}
        onCancel={() => setModalOpen(false)}
        okText="保存"
        cancelText="取消"
        destroyOnHidden
      >
        <Form<UserFormValues> form={form} layout="vertical" autoComplete="off">
          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, min: 3, max: 50, message: '用户名 3-50 个字符' }]}
          >
            {/* 用户名是身份标识，创建后不允许修改 */}
            <Input disabled={!!editing} />
          </Form.Item>

          {!editing && (
            <Form.Item
              name="password"
              label="初始密码"
              rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
          )}

          <Form.Item name="real_name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input />
          </Form.Item>

          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Radio.Group>
              {assignableRoles.map((r) => (
                <Radio.Button key={r} value={r}>
                  {ROLE_LABEL[r]}
                </Radio.Button>
              ))}
            </Radio.Group>
          </Form.Item>

          <Form.Item name="email" label="邮箱">
            <Input allowClear placeholder={editing ? '留空表示清除' : ''} />
          </Form.Item>

          <Form.Item name="phone" label="电话">
            <Input allowClear placeholder={editing ? '留空表示清除' : ''} />
          </Form.Item>

          {editing && (
            <Text type="secondary">角色上限：不能授予高于自己的角色（后端同样会校验）。</Text>
          )}
        </Form>
      </Modal>

      <Modal
        title={`修改密码：${pwdTarget?.username ?? ''}`}
        open={!!pwdTarget}
        onOk={submitPassword}
        onCancel={() => {
          setPwdTarget(null)
          pwdForm.resetFields()
        }}
        okText="修改"
        cancelText="取消"
        destroyOnHidden
      >
        <Form form={pwdForm} layout="vertical" autoComplete="off">
          <Form.Item
            name="old_password"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title={`重置密码：${resetTarget?.username ?? ''}`}
        open={!!resetTarget}
        onOk={submitReset}
        onCancel={() => {
          setResetTarget(null)
          resetForm.resetFields()
        }}
        okText="重置"
        okButtonProps={{ danger: true }}
        cancelText="取消"
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          管理员重置不需要旧密码。重置后请通过线下渠道把新密码告知本人，
          并提醒其登录后自行修改。
        </Typography.Paragraph>
        <Form form={resetForm} layout="vertical" autoComplete="off">
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
