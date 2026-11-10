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
  fetchQualityDistribution,
  type DefectCount,
  type GradeCount,
  type InspectorStat,
  type MachineOperator,
  type OperatorStat,
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
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (r: [Dayjs, Dayjs]) => {
    setLoading(true)
    try {
      // 两个看板可以并行拉取：它们互不依赖。
      const query = { start: r[0].toISOString(), end: r[1].toISOString() }
      const [q, m] = await Promise.all([
        fetchQualityDistribution(query),
        fetchMachineOperator(query),
      ])
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
