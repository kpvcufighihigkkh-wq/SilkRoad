import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Progress,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs, { type Dayjs } from 'dayjs'
import {
  fetchMachineOperator,
  fetchProductionOverview,
  fetchQualityDistribution,
  type DefectCount,
  type EdgeLotStat,
  type GradeCount,
  type InspectorStat,
  type LotStatusStat,
  type MachineOperator,
  type OperatorStat,
  type ProductTypeStat,
  type ProductionOverview,
  type QualityDistribution,
  type SpinningLineStat,
} from '@/api/reports'

const { Title } = Typography
const { RangePicker } = DatePicker

/** 后端缺省窗口是最近 7 天，这里与之保持一致，避免前后端口径不一致。 */
const DEFAULT_DAYS = 7

const gradeColumns: ColumnsType<GradeCount> = [
  { title: '等级', dataIndex: 'grade', render: (v: string) => <Tag>{v}</Tag> },
  {
    title: '数量',
    dataIndex: 'count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

const defectColumns: ColumnsType<DefectCount> = [
  { title: '缺陷原因', dataIndex: 'defect_reason' },
  {
    title: '次数',
    dataIndex: 'count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

const lineColumns: ColumnsType<SpinningLineStat> = [
  {
    title: '机台',
    dataIndex: 'spinning_line_id',
    render: (v: string) => <span className="mono">{v.slice(0, 8)}</span>,
  },
  {
    title: '落纱数',
    dataIndex: 'doffing_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '实际重量 (kg)',
    dataIndex: 'actual_weight',
    align: 'right',
    render: (v: number) => v.toLocaleString(undefined, { maximumFractionDigits: 1 }),
  },
]

const operatorColumns: ColumnsType<OperatorStat> = [
  {
    title: '操作员',
    dataIndex: 'operator_id',
    // null 组是「未记录操作员」的落纱，必须显式展示，不能丢。
    render: (v: string | null) =>
      v ? <span className="mono">{v.slice(0, 8)}</span> : <Tag color="default">未记录</Tag>,
  },
  {
    title: '落纱数',
    dataIndex: 'doffing_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

/** 批次状态的中文标签与颜色，与后端 lot.status 枚举一一对应。 */
const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  in_progress: { label: '生产中', color: 'processing' },
  paused: { label: '已暂停', color: 'warning' },
  completed: { label: '已完工', color: 'success' },
  cancelled: { label: '已取消', color: 'default' },
}

const statusColumns: ColumnsType<LotStatusStat> = [
  {
    title: '状态',
    dataIndex: 'status',
    render: (v: string) => {
      const s = STATUS_LABELS[v] ?? { label: v, color: 'default' }
      return <Tag color={s.color}>{s.label}</Tag>
    },
  },
  {
    title: '批次数',
    dataIndex: 'lot_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '计划量',
    dataIndex: 'planned_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '实际量',
    dataIndex: 'actual_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

const productColumns: ColumnsType<ProductTypeStat> = [
  { title: '产品类型', dataIndex: 'product_type', render: (v: string) => <Tag>{v}</Tag> },
  {
    title: '批次数',
    dataIndex: 'lot_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '计划量',
    dataIndex: 'planned_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '实际量',
    dataIndex: 'actual_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

const edgeColumns: ColumnsType<EdgeLotStat> = [
  {
    title: '来源设备',
    dataIndex: 'edge_id',
    // null 组是「未记录来源」的批次，必须显式展示，不能丢。
    render: (v: string | null) =>
      v ? <span className="mono">{v.slice(0, 8)}</span> : <Tag color="default">未记录</Tag>,
  },
  {
    title: '批次数',
    dataIndex: 'lot_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '计划量',
    dataIndex: 'planned_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
  {
    title: '实际量',
    dataIndex: 'actual_quantity',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

const inspectorColumns: ColumnsType<InspectorStat> = [
  {
    title: '检测员',
    dataIndex: 'inspector_id',
    render: (v: string | null) =>
      v ? <span className="mono">{v.slice(0, 8)}</span> : <Tag color="default">未记录</Tag>,
  },
  {
    title: '工检数',
    dataIndex: 'inspection_count',
    align: 'right',
    render: (v: number) => v.toLocaleString(),
  },
]

export default function ReportsPage() {
  // 区间状态用 Dayjs 保存，提交时再转 RFC3339。
  const [range, setRange] = useState<[Dayjs, Dayjs]>(() => [
    dayjs().subtract(DEFAULT_DAYS, 'day'),
    dayjs(),
  ])
  const [quality, setQuality] = useState<QualityDistribution | null>(null)
  const [machine, setMachine] = useState<MachineOperator | null>(null)
  const [production, setProduction] = useState<ProductionOverview | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (r: [Dayjs, Dayjs]) => {
    setLoading(true)
    try {
      // 三个看板可以并行拉取：它们互不依赖。
      const query = { start: r[0].toISOString(), end: r[1].toISOString() }
      const [p, q, m] = await Promise.all([
        fetchProductionOverview(query),
        fetchQualityDistribution(query),
        fetchMachineOperator(query),
      ])
      setProduction(p)
      setQuality(q)
      setMachine(m)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '加载报表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  // 进入页面拉取一次。按 spec §5.3 不做实时刷新，只提供手动刷新按钮。
  useEffect(() => {
    void load(range)
    // 仅在挂载时拉取；后续区间变化由用户点「查询」触发。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onSearch = () => {
    if (!range?.[0] || !range?.[1]) {
      message.warning('请选择时间范围')
      return
    }
    // 后端上限 366 天，超限会返回 400；提前拦下给出更明确提示。
    if (range[1].diff(range[0], 'day') > 366) {
      message.warning('时间跨度不得超过 366 天')
      return
    }
    void load(range)
  }

  const resetRange = () => {
    const next: [Dayjs, Dayjs] = [dayjs().subtract(DEFAULT_DAYS, 'day'), dayjs()]
    setRange(next)
    void load(next)
  }

  const qualified = quality?.qualified
  const ratePercent = qualified ? Math.round(qualified.qualified_rate * 1000) / 10 : 0

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card size="small">
        <Space wrap>
          <span>统计区间</span>
          <RangePicker
            showTime={{ format: 'HH:mm' }}
            format="YYYY-MM-DD HH:mm"
            value={range}
            onChange={(v) => {
              if (v?.[0] && v?.[1]) setRange([v[0], v[1]])
            }}
            allowClear={false}
          />
          <Button type="primary" onClick={onSearch} loading={loading}>
            查询
          </Button>
          <Button icon={<ReloadOutlined />} onClick={resetRange} loading={loading}>
            最近 7 天
          </Button>
        </Space>
      </Card>

      <Card title="生产概览" size="small" loading={loading}>
        {production && production.totals.lot_count > 0 ? (
          <>
            <Row gutter={16}>
              <Col xs={12} md={6}>
                <Statistic title="批次数" value={production.totals.lot_count} />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title="计划产量"
                  value={production.totals.planned_quantity}
                  valueStyle={{ color: '#595959' }}
                />
              </Col>
              <Col xs={12} md={6}>
                <Statistic title="实际产量" value={production.totals.actual_quantity} />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title="完成率"
                  value={Math.round(production.totals.completion_rate * 1000) / 10}
                  suffix="%"
                  valueStyle={{
                    color:
                      production.totals.completion_rate >= 0.95
                        ? '#3f8600'
                        : production.totals.completion_rate >= 0.7
                          ? '#d48806'
                          : '#cf1322',
                  }}
                />
              </Col>
            </Row>

            <Row gutter={16} style={{ marginTop: 16 }}>
              <Col xs={24} lg={8}>
                <Title level={5}>按状态</Title>
                <Table<LotStatusStat>
                  rowKey="status"
                  columns={statusColumns}
                  dataSource={production.by_status}
                  size="small"
                  pagination={false}
                />
              </Col>
              <Col xs={24} lg={8}>
                <Title level={5}>按产品类型</Title>
                <Table<ProductTypeStat>
                  rowKey="product_type"
                  columns={productColumns}
                  dataSource={production.by_product}
                  size="small"
                  pagination={false}
                />
              </Col>
              <Col xs={24} lg={8}>
                <Title level={5}>按来源设备</Title>
                <Table<EdgeLotStat>
                  // 分组键可空，rowKey 必须能唯一标识 null 组。
                  rowKey={(r) => r.edge_id ?? '__null__'}
                  columns={edgeColumns}
                  dataSource={production.by_edge}
                  size="small"
                  pagination={false}
                />
              </Col>
            </Row>
          </>
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="区间内无批次。批次由 Edge 投产时创建，若长期为空请检查 Edge 是否已上报。"
          />
        )}
      </Card>

      <Row gutter={16}>
        <Col xs={24} xl={12}>
          <Card title="质量分布" size="small" loading={loading}>
            {qualified ? (
              <>
                <Row gutter={16}>
                  <Col span={8}>
                    <Statistic
                      title="合格率"
                      value={ratePercent}
                      suffix="%"
                      // 口径提示：分母排除「未判定」，避免用户误以为不合格偏多。
                      valueStyle={{
                        color: ratePercent >= 90 ? '#3f8600' : ratePercent >= 70 ? '#d48806' : '#cf1322',
                      }}
                    />
                  </Col>
                  <Col span={8}>
                    <Statistic title="已判定" value={qualified.qualified_count + qualified.unqualified_count} />
                  </Col>
                  <Col span={8}>
                    <Statistic
                      title="未判定"
                      value={qualified.unspecified_count}
                      valueStyle={{ color: qualified.unspecified_count > 0 ? '#d48806' : undefined }}
                    />
                  </Col>
                </Row>

                <div style={{ margin: '12px 0' }}>
                  <Progress
                    percent={ratePercent}
                    success={{ percent: ratePercent }}
                    format={() => `${qualified.qualified_count} 合格 / ${qualified.unqualified_count} 不合格`}
                  />
                </div>

                <Title level={5} style={{ marginTop: 16 }}>
                  等级分布
                </Title>
                {quality && quality.by_grade.length > 0 ? (
                  <Table<GradeCount>
                    rowKey="grade"
                    columns={gradeColumns}
                    dataSource={quality.by_grade}
                    size="small"
                    pagination={false}
                  />
                ) : (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="区间内无质检记录" />
                )}

                <Title level={5} style={{ marginTop: 16 }}>
                  缺陷原因 Top {quality?.top_defects.length || 10}
                </Title>
                {quality && quality.top_defects.length > 0 ? (
                  <Table<DefectCount>
                    rowKey="defect_reason"
                    columns={defectColumns}
                    dataSource={quality.top_defects}
                    size="small"
                    pagination={false}
                  />
                ) : (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="区间内无缺陷原因记录" />
                )}
              </>
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />
            )}
          </Card>
        </Col>

        <Col xs={24} xl={12}>
          <Card title="机台与操作员" size="small" loading={loading}>
            <Title level={5}>按机台产量</Title>
            {machine && machine.by_spinning_line.length > 0 ? (
              <Table<SpinningLineStat>
                rowKey="spinning_line_id"
                columns={lineColumns}
                dataSource={machine.by_spinning_line}
                size="small"
                pagination={false}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="区间内无落纱记录" />
            )}

            <Title level={5} style={{ marginTop: 16 }}>
              按操作员落纱数
            </Title>
            {machine && machine.by_operator.length > 0 ? (
              <Table<OperatorStat>
                // 分组键可空，rowKey 必须能唯一标识 null 组。
                rowKey={(r) => r.operator_id ?? '__null__'}
                columns={operatorColumns}
                dataSource={machine.by_operator}
                size="small"
                pagination={false}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="区间内无落纱记录" />
            )}

            <Title level={5} style={{ marginTop: 16 }}>
              按检测员工检数
            </Title>
            {machine && machine.by_inspector.length > 0 ? (
              <Table<InspectorStat>
                rowKey={(r) => r.inspector_id ?? '__null__'}
                columns={inspectorColumns}
                dataSource={machine.by_inspector}
                size="small"
                pagination={false}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="区间内无质检记录" />
            )}
          </Card>
        </Col>
      </Row>

      <Card size="small">
        <Typography.Text type="secondary">
          统计口径：合格率按质检记录行计（复检各计一次），分母不含「未判定」；落纱统计已排除作废记录。
          {quality ? ` 当前区间 ${dayjs(quality.range.start).format('YYYY-MM-DD HH:mm')} ~ ${dayjs(quality.range.end).format('YYYY-MM-DD HH:mm')}。` : ''}
        </Typography.Text>
      </Card>
    </Space>
  )
}
