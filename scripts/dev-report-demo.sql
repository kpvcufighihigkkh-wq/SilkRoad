-- 报表页演示数据（仅用于本地开发库验证，不入库版本控制）
-- 目的：让质量分布与机台/操作员两个看板有可核对的数字，
-- 覆盖三态合格率、复检、缺陷原因、可空分组键与作废落纱。

BEGIN;

-- 8 条质检记录：4 合格 / 2 不合格 / 2 未判定（NULL）
-- 其中 bobbin b-1 复检两次（不合格 -> 合格），用于验证「按行计数」口径。
INSERT INTO bobbin_grades
  (id, bobbin_id, inspector_id, grade, is_qualified, defect_reason, inspected_at, created_at, updated_at)
VALUES
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000001', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'C', false, '毛丝',     now() - interval '5 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000001', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'A', true,  NULL,       now() - interval '4 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000002', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'A', true,  NULL,       now() - interval '4 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000003', NULL,                                   'B', true,  NULL,       now() - interval '3 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000004', NULL,                                   'D', false, '断头',     now() - interval '3 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000005', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'B', true,  NULL,       now() - interval '2 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000006', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'B', NULL,  NULL,       now() - interval '1 days', now(), now()),
  (gen_random_uuid(), 'aaaaaaaa-0000-4000-8000-000000000007', '6de8a73a-7aef-40d4-acec-e541cd5456c8', 'C', NULL,  '油污',     now() - interval '1 days', now(), now());

-- 6 条落纱：Line 1 三条（含一条作废）、Line 2 两条、Line 3 一条
-- 作废那条必须不出现在机台统计里，用于验证后端 §3.2.2 的口径。
INSERT INTO doffings
  (id, spinning_line_id, spinning_position, operator_id, status, actual_weight, doffing_time, created_at, updated_at, lot_id)
VALUES
  (gen_random_uuid(), 'e2693c32-69b6-442d-8f33-ba7e8b51e05d', 1, '49827a6e-a1ac-4bc3-8e86-d8b1217ea661', 'confirmed', 5.2,  now() - interval '5 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43'),
  (gen_random_uuid(), 'e2693c32-69b6-442d-8f33-ba7e8b51e05d', 2, '49827a6e-a1ac-4bc3-8e86-d8b1217ea661', 'confirmed', 5.4,  now() - interval '4 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43'),
  (gen_random_uuid(), 'e2693c32-69b6-442d-8f33-ba7e8b51e05d', 3, NULL,                                   'cancelled', NULL, now() - interval '4 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43'),
  (gen_random_uuid(), '0bef2ad0-b58b-4ff3-b90f-5b0cb02ed208', 1, '49827a6e-a1ac-4bc3-8e86-d8b1217ea661', 'pending',   5.1,  now() - interval '3 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43'),
  (gen_random_uuid(), '0bef2ad0-b58b-4ff3-b90f-5b0cb02ed208', 2, NULL,                                   'confirmed', NULL, now() - interval '2 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43'),
  (gen_random_uuid(), '79506cdb-dd20-4b43-a41a-4eb737165502', 1, '49827a6e-a1ac-4bc3-8e86-d8b1217ea661', 'confirmed', 5.0,  now() - interval '1 days', now(), now(), '30e759bd-e9a3-46b0-ae5f-59e5a5489d43');

-- 5 条丝锭，用于批次详情页的丝锭列表。
-- 覆盖四种状态与「等级为空」的展示（空值应显示为 —，而非空白）。
INSERT INTO bobbins
  (id, bobbin_number, lot_id, spinning_position, gross_weight, net_weight, tare_weight,
   grade, status, label_printed, created_at, updated_at)
VALUES
  (gen_random_uuid(), 'FDY-2026-001-01-B01', '30e759bd-e9a3-46b0-ae5f-59e5a5489d43', 1, 5.45, 5.10, 0.35, 'A',    'completed', true,  now() - interval '5 days', now()),
  (gen_random_uuid(), 'FDY-2026-001-01-B02', '30e759bd-e9a3-46b0-ae5f-59e5a5489d43', 2, 5.52, 5.17, 0.35, 'B',    'packed',    true,  now() - interval '4 days', now()),
  (gen_random_uuid(), 'FDY-2026-001-01-B03', '30e759bd-e9a3-46b0-ae5f-59e5a5489d43', 3, 5.38, 5.03, 0.35, 'C',    'shipped',   true,  now() - interval '3 days', now()),
  (gen_random_uuid(), 'FDY-2026-001-01-B04', '30e759bd-e9a3-46b0-ae5f-59e5a5489d43', 4, 5.60, 5.25, 0.35, 'A',    'completed', true,  now() - interval '2 days', now()),
  (gen_random_uuid(), 'FDY-2026-001-01-B05', '30e759bd-e9a3-46b0-ae5f-59e5a5489d43', 5, 5.44, 5.09, 0.35, NULL,   'producing', false, now() - interval '1 days', now());

COMMIT;
