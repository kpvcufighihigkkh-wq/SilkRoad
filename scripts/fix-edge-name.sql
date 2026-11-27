-- 修复：端到端测试脚本写入的设备名称乱码
-- 背景：edge-e2e-pallet 一行的 edge_name 为非 UTF-8 编码写入，前端显示为问号/方块。
-- 该行是测试数据，改名不影响业务；生产库执行前请先按 edge_code 确认。
UPDATE edges
SET edge_name = '二号边端（端到端验证）',
    updated_at = now()
WHERE edge_code = 'edge-e2e-pallet';
