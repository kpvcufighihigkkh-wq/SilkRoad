"""生成《系统功能与对外接口》Excel（按「对外提供的接口内容」组织，不列数据库结构）。

数据来源：
  - 页面功能：实拍截图 + 各页面源码
  - 接口内容：本机 Center 实例的真实返回（2026-11-15），字段名与示例值均为实际值
运行：python scripts/gen_delivery_xlsx.py
"""

import os
from openpyxl import Workbook
from openpyxl.drawing.image import Image as XLImage
from openpyxl.styles import Alignment, Border, Font, PatternFill, Side
from openpyxl.utils import get_column_letter

OUT = os.path.join("docs", "delivery", "系统接口汇总_2026-11-15_v2.xlsx")


def resolve_out(path):
    """目标文件被 Excel 打开时改名另存，避免覆盖失败丢结果。"""
    try:
        with open(path, "ab"):
            pass
        return path
    except PermissionError:
        stem, ext = os.path.splitext(path)
        for i in range(2, 10):
            candidate = f"{stem}_v{i}{ext}"
            try:
                with open(candidate, "ab"):
                    pass
                return candidate
            except PermissionError:
                continue
        raise
SHOTS = os.path.join("docs", "delivery", "screenshots")

HEAD_FILL = PatternFill("solid", fgColor="1F4E79")
SUB_FILL = PatternFill("solid", fgColor="D9E2F3")
HEAD_FONT = Font(color="FFFFFF", bold=True, size=11)
TITLE_FONT = Font(bold=True, size=13)
THIN = Side(style="thin", color="BFBFBF")
BORDER = Border(left=THIN, right=THIN, top=THIN, bottom=THIN)
WRAP = Alignment(wrap_text=True, vertical="top")

# 统一的表头：阶段接口一律按「字段 / 类型 / 是否必填 / 示例值 / 说明」组织。
FIELD_HEADERS = ["序号", "字段业务名称", "字段名", "类型", "长度/精度", "是否必填", "示例值", "字段说明"]
FIELD_WIDTHS = [6, 20, 22, 14, 12, 10, 34, 62]


def write_header(ws, row, headers, widths):
    for c, h in enumerate(headers, start=1):
        cell = ws.cell(row=row, column=c, value=h)
        cell.fill = HEAD_FILL
        cell.font = HEAD_FONT
        cell.alignment = Alignment(wrap_text=True, vertical="center", horizontal="center")
        cell.border = BORDER
        ws.column_dimensions[get_column_letter(c)].width = widths[c - 1]
    ws.row_dimensions[row].height = 28


def write_rows(ws, start_row, rows, width_for_wrap=8):
    r = start_row
    for row in rows:
        for c, v in enumerate(row, start=1):
            cell = ws.cell(row=r, column=c, value=v)
            cell.alignment = WRAP
            cell.border = BORDER
        r += 1
    return r


def stage_sheet(wb, title, subtitle, fields, example_json, note=None):
    """一个阶段一张表：字段清单 + 真实报文示例。"""
    ws = wb.create_sheet(title)
    ws["A1"] = subtitle
    ws["A1"].font = TITLE_FONT
    ws.row_dimensions[1].height = 26

    write_header(ws, 3, FIELD_HEADERS, FIELD_WIDTHS)
    r = write_rows(ws, 4, fields)

    if note:
        r += 1
        ws.cell(row=r, column=1, value="说明").font = Font(bold=True)
        r += 1
        cell = ws.cell(row=r, column=1, value=note)
        cell.alignment = Alignment(wrap_text=True, vertical="top")
        ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=len(FIELD_HEADERS))
        ws.row_dimensions[r].height = 46

    if example_json:
        r += 2
        ws.cell(row=r, column=1, value="接口返回示例（本机真实数据）").font = Font(bold=True)
        r += 1
        ws.cell(row=r, column=1, value=example_json).alignment = Alignment(
            wrap_text=True, vertical="top"
        )
        ws.merge_cells(start_row=r, start_column=1, end_row=r, end_column=len(FIELD_HEADERS))
        ws.row_dimensions[r].height = 20 * example_json.count("\n") + 20

    ws.freeze_panes = "A4"
    return ws


def main():
    wb = Workbook()
    wb.remove(wb.active)

    # ============ 说明 ============
    ws = wb.create_sheet("说明")
    ws["A1"] = "IGH SilkRoad 系统功能与对外接口"
    ws["A1"].font = Font(bold=True, size=16)
    lines = [
        "",
        "版本：2026-11-15（数据为演示库真实内容，非 mock 占位）",
        "",
        "一、本文件的组织方式",
        "   按「业务阶段」组织，每个阶段说明系统在这一步能对外提供什么数据：",
        "     阶段 0  批次开工：批次建立后可提供的基础信息",
        "     阶段 1  落筒（Doffing）：按一次落筒提供，含 12 枚丝饼明细",
        "     阶段 2  称重与质检：单枚丝饼的重量与质检结论、五维评分、质检项",
        "     阶段 3  丝车（Trolley）：一次装车对应的落筒集合 + 整车重量 + 车辆状态",
        "     阶段 4  托盘（Pallet）：整托信息 + 盘内明细（不带丝车与落筒号）",
        "   另有：接口清单（Center / Edge）、页面功能说明（含各角色截图）、层级核对、已知问题。",
        "",
        "二、所有接口共用的响应格式",
        '   成功：{"code":0,"message":"success","data":...}',
        '   分页：{"code":0,"message":"success","data":[...],"pagination":{"page":1,"page_size":20,"total":108,"total_pages":6}}',
        '   失败：{"code":<错误码>,"message":"<说明>"}',
        "",
        "三、角色与可见范围",
        "   管理员 admin      ：全部页面 + 系统管理（用户、设备）",
        "   操作员 operator1  ：批次、托盘、落纱桶、纸箱、吊车、报表（密码 password）",
        "   质检员 inspector1 ：同上（密码 password）",
        "   只读用户 viewer1  ：同上（密码 password）",
        "   系统管理菜单按角色隐藏；后端 /v1/users、/v1/edges 仍为 admin-only。",
        "",
        "四、当前系统对外提供的数据范围（一句话版）",
        "   已提供：批次、落筒、丝饼、称重、质检（含五维分与质检项）、落纱桶、纸箱、吊车、托盘、三类报表。",
        "   未提供：丝车（Trolley）独立实体、整车称重接口、落筒分组（一次 12 枚的分组号）。",
        "   详见「层级核对」工作表。",
    ]
    for i, text in enumerate(lines, start=2):
        ws.cell(row=i, column=1, value=text)
    ws.column_dimensions["A"].width = 118

    # ============ 阶段 0：批次开工 ============
    stage_sheet(
        wb,
        "阶段0-批次开工",
        "阶段 0：批次开工（Lot）—— Edge 开工时创建，Center 只读",
        [
            [1, "批次号", "lot_number", "string", "50", "是", "FDY-2026-002-01", "我方生成的批次号，跨系统唯一标识"],
            [2, "PLC 批次号", "plc_lot_number", "string", "50", "否", "PLC-2026-002", "现场 PLC / 上位系统里的批次号，用于对账"],
            [3, "订单号", "order_code", "string", "50", "否", "SO-2026-1101", "客户销售订单号"],
            [4, "产品类型", "product_type", "enum", "—", "是", "FDY", "FDY / POY / DTY，决定后续是否走纸箱"],
            [5, "规格", "product_spec", "string", "50", "否", "150D/48F", "产品规格（旦数/孔数）"],
            [6, "状态", "status", "enum", "—", "是", "in_progress", "in_progress 生产中 / paused 已暂停 / completed 已完工 / cancelled 已取消"],
            [7, "计划产量", "planned_quantity", "int", "10", "是", "4800", "本批次计划生产的丝饼枚数"],
            [8, "实际产量", "actual_quantity", "int", "10", "是", "108", "已完成的丝饼枚数，由落筒推进"],
            [9, "进度", "progress", "number", "—", "是", "2", "百分比，后端计算，前端不重算"],
            [10, "开工时间", "start_time", "datetime", "23", "否", "2026-11-14T01:03:00Z", "Edge 投产时刻，报表按它做区间过滤"],
            [11, "结束时间", "end_time", "datetime", "23", "否", "—", "完工时刻，状态流转时写入"],
            [12, "来源设备", "edge_id", "uuid", "36", "否", "f707a7a3-d840-4645-b1fc-0fe60e0ead9d", "产生该批次的 Edge 设备 ID"],
        ],
        """{
  "code": 0, "message": "success",
  "data": [{
    "id": "7f1b3d8b-99b7-4b6e-9aec-2322fb50a722",
    "lot_number": "FDY-2026-002-01",
    "edge_id": "f707a7a3-d840-4645-b1fc-0fe60e0ead9d",
    "plc_lot_number": "PLC-2026-002",
    "order_code": "SO-2026-1101",
    "product_type": "FDY", "product_spec": "150D/48F",
    "status": "in_progress",
    "planned_quantity": 4800, "actual_quantity": 108, "progress": 2,
    "start_time": "2026-11-14T01:03:00Z",
    "created_at": "2026-11-15T03:03:08Z", "updated_at": "2026-11-15T03:03:08Z"
  }]
}""",
        "批次由 Edge 创建并上传，Center 不提供写接口 —— 数据归属单向，回写会破坏可追溯性。",
    )

    # ============ 阶段 1：落筒 ============
    stage_sheet(
        wb,
        "阶段1-落筒Doffing",
        "阶段 1：落筒（Doffing）—— 一次落筒 = 1 条主记录 + 12 枚丝饼",
        [
            [1, "落筒号", "id", "uuid", "36", "是", "c9750ba1-9d13-440d-ad69-f19185a8c933", "我方生成的落筒记录号（UUID）"],
            [2, "客户线号", "spinning_line_id", "uuid", "36", "是", "e2693c32-69b6-442d-8f33-ba7e8b51e05d", "线体 ID；对接层可换成客户线号（如 L7 / CP6）"],
            [3, "客户位号", "spinning_position", "int", "10", "是", "9", "纺位/位号，1..N"],
            [4, "批次号", "lot_id", "uuid", "36", "是", "7f1b3d8b-99b7-4b6e-9aec-2322fb50a722", "所属批次；对接时同时附 lot_number 与 product_spec"],
            [5, "落筒时间", "doffing_time", "datetime", "23", "是", "2026-11-14T09:03:00Z", "现场落筒时刻（UTC，展示时转本地）"],
            [6, "确认时间", "confirmed_at", "datetime", "23", "否", "2026-11-14T09:05:00Z", "操作员确认时刻"],
            [7, "落筒状态", "status", "enum", "—", "是", "confirmed", "pending 待确认 / confirmed 已确认 / cancelled 已作废"],
            [8, "丝饼号", "bobbin_number", "string", "50", "否", "FDY-2026-002-01-B09", "确认时回填，对应本次落筒的主丝饼"],
            [9, "实际重量", "actual_weight", "decimal", "18,3", "否", "14.99", "确认时回填，单位 kg"],
            [10, "等级", "grade", "string", "10", "否", "B", "确认时回填的等级，质检后以质检记录为准"],
            [11, "操作员", "operator_id", "uuid", "36", "否", "（空）", "落筒操作员；未登录 PDA 时为空"],
            [12, "作废原因", "cancel_reason", "text", "—", "否", "—", "status=cancelled 时必填"],
            [13, "丝饼明细", "bobbin_list", "array", "—", "否", "见阶段 2", "12 枚丝饼的 bobbin_id 列表；一次落筒的完整内容"],
        ],
        """{
  "code": 0, "message": "success",
  "data": [{
    "id": "c9750ba1-9d13-440d-ad69-f19185a8c933",
    "spinning_line_id": "e2693c32-69b6-442d-8f33-ba7e8b51e05d",
    "spinning_position": 9,
    "lot_id": "7f1b3d8b-99b7-4b6e-9aec-2322fb50a722",
    "status": "confirmed",
    "bobbin_number": "FDY-2026-002-01-B09",
    "actual_weight": 14.99, "grade": "B",
    "doffing_time": "2026-11-14T09:03:00Z",
    "confirmed_at": "2026-11-14T09:05:00Z",
    "created_at": "2026-11-15T03:03:08Z", "updated_at": "2026-11-15T03:03:08Z"
  }],
  "pagination": {"page":1,"page_size":20,"total":9,"total_pages":1}
}""",
        "对接要点：现模型一次 doffing 记录对应一个锭位；客户口径「一次落筒 12 枚」需要按"
        "「线号 + 位号 + 落筒时间窗」聚合，或新增落筒分组实体。见「层级核对」。",
    )

    # ============ 阶段 2：称重与质检 ============
    stage_sheet(
        wb,
        "阶段2-丝饼称重质检",
        "阶段 2：丝饼（Bobbin）称重与质检 —— 可对外提供的最小追溯单元",
        [
            [1, "丝饼号", "bobbin_number", "string", "50", "是", "FDY-2026-002-01-D09-B12", "条码/丝饼唯一号，客户扫码即此值"],
            [2, "批次", "lot_id", "uuid", "36", "是", "7f1b3d8b-99b7-4b6e-9aec-2322fb50a722", "所属批次"],
            [3, "锭位", "spinning_position", "int", "10", "是", "12", "纺位号"],
            [4, "毛重", "gross_weight", "decimal", "18,3", "是", "15.055", "kg，称重设备给出"],
            [5, "净重", "net_weight", "decimal", "18,3", "是", "14.635", "kg，毛重 − 皮重"],
            [6, "皮重", "tare_weight", "decimal", "18,3", "是", "0.42", "kg，纸管重量"],
            [7, "等级", "grade", "string", "20", "否", "C", "A/B/C/D"],
            [8, "丝饼状态", "status", "enum", "—", "是", "packed", "producing 生产中 / completed 已完成 / packed 已包装 / shipped 已发货"],
            [9, "落筒时间", "completed_at", "datetime", "23", "否", "2026-11-14T09:03:00Z", "该丝饼的落筒时刻"],
            [10, "所属桶", "barrel_id", "uuid", "36", "否", "142ec792-1b93-4502-b113-ce034e30dc0f", "null 表示未装桶"],
            [11, "所属纸箱", "carton_id", "uuid", "36", "否", "（null）", "DTY 专属，null 表示未装箱"],
            [12, "所属托盘", "pallet_id", "uuid", "36", "否", "a7af073c-6fa3-4503-955a-a53fa786652d", "null 表示未码盘"],
            [13, "标签打印", "label_printed", "boolean", "—", "是", "false", "标签是否已打印"],
            [14, "质检结论", "is_qualified", "boolean", "—", "否", "false", "三态：true 合格 / false 不合格 / null 未判定（未判定 ≠ 不合格）"],
            [15, "质检等级", "grade（质检记录）", "string", "20", "是", "C", "质检记录里的等级，可能覆盖落筒时的初判"],
            [16, "综合得分", "final_score", "number", "—", "否", "71.5", "0–100，未录入时为 null"],
            [17, "五维评分", "scores", "json", "—", "否", "见示例", "外观/强度/伸长/含油/线密度五项分值"],
            [18, "质检项判定", "inspection_details", "json", "—", "否", "见示例", "每项 pass / fail / na 三态，报表据此算分项合格率"],
            [19, "缺陷原因", "defect_reason", "text", "—", "否", "毛丝", "不合格原因，用于缺陷 Top N 统计"],
            [20, "质检员", "inspector_id", "uuid", "36", "否", "f45860ae-56aa-4310-9cd8-02d2468d0f97", "null 显示为「未记录」"],
            [21, "质检时间", "inspected_at", "datetime", "23", "是", "2026-11-14T09:11:00Z", "报表按它做区间统计"],
        ],
        """{
  "code": 0, "message": "success",
  "data": [{
    "id": "159d7303-af77-4b82-a2d4-9286efdfc25f",
    "bobbin_id": "84cbf223-7faf-483a-9ca3-fff047853401",
    "inspector_id": "f45860ae-56aa-4310-9cd8-02d2468d0f97",
    "grade": "C", "final_score": 71.5,
    "scores": {"linear_density":92,"strength":88,"elongation":90,"oil_pickup":95,"appearance":62},
    "inspection_details": {"appearance":"fail","broken_end":"pass","hairiness":"fail","oil_stain":"pass","weight":"pass"},
    "is_qualified": false, "defect_reason": "毛丝",
    "inspected_at": "2026-11-14T09:11:00Z"
  }]
}""",
        "称重与质检是两个独立动作：PUT /v1/bobbins/:id/weigh 与 PUT /v1/bobbins/:id/inspect。"
        "质检记录只增不改，复检会产生新记录（报表按行计数）。",
    )

    # ============ 阶段 3：丝车 ============
    stage_sheet(
        wb,
        "阶段3-丝车Trolley",
        "阶段 3：丝车（Trolley）—— 当前以「吊车 + 落纱桶」承载，字段为可对外提供的映射",
        [
            [1, "丝车号", "module_number", "string", "50", "是", "MDL-POY-2026-003-01-04", "客户 RFID 车号 / 我方生成的车编号"],
            [2, "丝车 ID", "id", "uuid", "36", "是", "54d85669-aa7e-4de0-af18-364ae04272f7", "我方生成的车唯一号"],
            [3, "落筒 1", "barrel1_id → doffings", "uuid", "36", "是", "c6de7333-021f-4f53-8b3b-bf05fb436d68", "现有模型：车挂 2 桶，每桶 9 枚；POY 2 落筒 / FDY 3 落筒需对接层聚合"],
            [4, "落筒 2", "barrel2_id → doffings", "uuid", "36", "否", "c7303f63-0433-4213-a89b-b7a59dda53d9", "第二桶；单桶车时为空"],
            [5, "装车时间", "created_at", "datetime", "23", "是", "2026-11-15T03:03:10Z", "建车时刻，相当于开始装车"],
            [6, "车辆状态", "status", "enum", "—", "是", "warehouse", "loading 装车中 / transporting 转运中 / sorting 分拣中 / warehouse 已入库 / idle 空闲"],
            [7, "RFID", "rfid", "string", "50", "否", "E200-DEMO-004", "车内 RFID 标签值"],
            [8, "当前位置", "current_location", "string", "100", "否", "A区4号货道", "工段/货道"],
            [9, "批号", "（经由 barrel → lot）", "string", "50", "否", "POY-2026-003-01", "冗余携带，保持精准"],
            [10, "丝饼数", "（桶装载量之和）", "int", "10", "否", "18", "车内实际丝饼数 = 各桶 current_count 之和"],
            [11, "整车毛重", "—", "decimal", "18,3", "否", "—", "当前无接口；见「层级核对」待补项"],
            [12, "整车净重", "—", "decimal", "18,3", "否", "—", "当前无接口；需新增整车称重"],
            [13, "剩余丝饼", "—", "int", "10", "否", "—", "客户口径 Remain 状态；现可派生为「桶容量 − 装载量」"],
        ],
        """{
  "code": 0, "message": "success",
  "data": [{
    "id": "54d85669-aa7e-4de0-af18-364ae04272f7",
    "module_number": "MDL-POY-2026-003-01-04",
    "barrel1_id": "c6de7333-021f-4f53-8b3b-bf05fb436d68",
    "barrel2_id": "c7303f63-0433-4213-a89b-b7a59dda53d9",
    "status": "warehouse",
    "rfid": "E200-DEMO-004",
    "current_location": "A区4号货道",
    "created_at": "2026-11-15T03:03:10Z", "updated_at": "2026-11-15T03:03:10Z"
  }]
}""",
        "重要：现有系统没有独立的「丝车」表。最接近的实体是吊车（module，挂 2 个落纱桶）"
        "与落纱桶（barrel，容量 1–9）。若客户口径要求「一车 2/3 个落筒、整车称重、Empty/Full/Remain」，"
        "需要新增丝车实体与整车称重接口，而不是靠字段映射。",
    )

    # ============ 阶段 4：托盘 ============
    stage_sheet(
        wb,
        "阶段4-托盘Pallet",
        "阶段 4：托盘（Pallet）—— 整托可对外提供的内容（明细不带丝车与落筒号）",
        [
            [1, "托盘号", "pallet_code", "string", "50", "是", "PLT-DTY-2026-004-01-01", "托盘唯一编号"],
            [2, "批次", "lot_id", "uuid", "36", "是", "4917ac5f-f134-461f-824c-6cd29c6e59e3", "所属批次"],
            [3, "托盘类型", "pallet_type", "enum", "—", "是", "carton", "bobbin 码丝饼（POY/FDY）/ carton 码纸箱（DTY）"],
            [4, "层级", "level", "int", "10", "是", "1", "栈板层级 1–9"],
            [5, "丝饼数", "bobbins_count", "int", "10", "是", "48", "整托丝饼总数"],
            [6, "纸箱数", "cartons_count", "int", "10", "是", "4", "DTY 类型时为箱数，其余为 0"],
            [7, "净重", "net_weight", "decimal", "18,2", "否", "718.81", "kg；null 表示未称重（≠ 0 kg）"],
            [8, "毛重", "gross_weight", "decimal", "18,2", "否", "743.81", "kg"],
            [9, "皮重", "tare_weight", "decimal", "18,2", "否", "25", "kg，托盘自重"],
            [10, "状态", "status", "enum", "—", "是", "completed", "building 码垛中 / completed 已封托 / shipped 已发货"],
            [11, "封托时间", "completed_at", "datetime", "23", "否", "2026-11-12T10:03:00Z", "整托完成时刻"],
            [12, "标签打印", "printed", "boolean", "—", "是", "false", "标签是否已打印"],
            [13, "来源设备", "edge_id", "uuid", "36", "否", "f707a7a3-d840-4645-b1fc-0fe60e0ead9d", "产生该托盘的 Edge"],
            [14, "盘内明细", "bobbins / cartons", "array", "—", "否", "见说明", "码锭类给 bobbin_number 列表；码箱类给 carton_number 列表"],
            [15, "客户 ERP 字段对齐", "Djguid / TM / BZPH / GG / DJ / JZ / MZ / TS / NWX / SFJLK", "—", "—", "否", "见说明", "与客户 BZT2H 表字段的对应关系，详见 Markdown 文档 4.3 节"],
        ],
        """{
  "code": 0, "message": "success",
  "data": [{
    "id": "b276e110-eb7a-4522-ba43-f493f65ee523",
    "pallet_code": "PLT-DTY-2026-004-01-01",
    "lot_id": "4917ac5f-f134-461f-824c-6cd29c6e59e3",
    "edge_id": "f707a7a3-d840-4645-b1fc-0fe60e0ead9d",
    "level": 1, "pallet_type": "carton",
    "bobbins_count": 48, "cartons_count": 4,
    "status": "completed",
    "net_weight": 718.81, "gross_weight": 743.81, "tare_weight": 25,
    "printed": false, "printed_at": null,
    "completed_at": "2026-11-12T10:03:00Z",
    "created_at": "2026-11-15T03:03:11Z", "updated_at": "2026-11-15T03:03:11Z"
  }]
}""",
        "托盘明细不带丝车号与落筒号 —— 这是设计选择：托盘是发给客户/ERP 的出货单元，"
        "带上层内部号会让客户侧被迫理解我方过程结构。需要溯源时由 BobbinID 反查。",
    )

    # ============ 接口清单 ============
    headers = ["方法", "路径", "角色", "用途", "返回内容（data）"]
    widths = [8, 46, 16, 40, 66]
    rows = [
        ["GET", "/health", "公开", "健康检查", "edge_id, online, status, type"],
        ["POST", "/v1/login", "公开", "登录", "token, 用户信息"],
        ["GET", "/v1/lots", "已登录", "批次列表", "批次列表 + pagination"],
        ["GET", "/v1/lots/:id", "已登录", "批次详情", "单个批次"],
        ["GET", "/v1/bobbins", "已登录", "丝饼列表", "丝饼列表 + pagination"],
        ["GET", "/v1/bobbins/:id", "已登录", "丝饼详情", "单个丝饼"],
        ["GET", "/v1/grades", "已登录", "质检记录", "质检列表（含五维分、质检项）+ pagination"],
        ["GET", "/v1/doffings", "已登录", "落筒记录", "落筒列表 + pagination"],
        ["GET", "/v1/pallets", "已登录", "托盘列表", "托盘列表 + pagination"],
        ["GET", "/v1/pallets/:id", "已登录", "托盘详情", "单个托盘"],
        ["GET", "/v1/barrels", "已登录", "落纱桶列表", "桶列表 + pagination"],
        ["GET", "/v1/barrels/:id", "已登录", "桶详情", "单个桶"],
        ["GET", "/v1/cartons", "已登录", "纸箱列表", "箱列表 + pagination"],
        ["GET", "/v1/cartons/:id", "已登录", "纸箱详情", "单个纸箱"],
        ["GET", "/v1/modules", "已登录", "吊车列表", "吊车列表 + pagination"],
        ["GET", "/v1/modules/:id", "已登录", "吊车详情", "单台吊车"],
        ["GET", "/v1/reports/production-overview", "已登录", "生产概览报表", "totals / by_status / by_product / by_edge"],
        ["GET", "/v1/reports/quality-distribution", "已登录", "质量分布报表", "qualified / by_grade / top_defects / by_dimension / average_score"],
        ["GET", "/v1/reports/machine-operator", "已登录", "机台与操作员报表", "by_spinning_line / by_operator / by_inspector"],
        ["POST", "/v1/users", "admin", "创建用户", "新用户"],
        ["GET", "/v1/users", "admin", "用户列表", "用户列表 + pagination"],
        ["GET", "/v1/users/:id", "admin", "用户详情", "单个用户"],
        ["PUT", "/v1/users/:id", "admin", "更新用户", "更新后的用户"],
        ["DELETE", "/v1/users/:id", "admin", "删除用户", "—"],
        ["PUT", "/v1/users/:id/password/reset", "admin", "重置他人密码", "—"],
        ["PUT", "/v1/users/:id/password", "admin/本人", "修改自己的密码", "—"],
        ["GET", "/v1/users/me", "已登录", "当前用户", "本人信息（前端据此渲染菜单）"],
        ["POST", "/v1/edges", "admin", "注册边端设备", "新设备"],
        ["GET", "/v1/edges", "admin", "设备列表", "设备列表 + pagination"],
        ["GET", "/v1/edges/:code", "admin", "设备详情", "单台设备"],
        ["POST", "/v1/edges/:code/token", "admin", "签发设备凭证", "edge token"],
        ["POST", "/v1/edges/:code/upload", "Edge Token", "接收 Edge 上传", "applied / rejected / errors / new_cursor"],
        ["GET", "/v1/edges/:code/base-data", "Edge Token", "向 Edge 下发基础数据", "线体、等级字典"],
    ]
    ws = wb.create_sheet("接口清单-Center")
    ws["A1"] = "Center 对外接口（HTTP/JSON，鉴权 Bearer Token）"
    ws["A1"].font = TITLE_FONT
    write_header(ws, 3, headers, widths)
    write_rows(ws, 4, rows)
    ws.freeze_panes = "A4"

    edge_rows = [
        ["POST", "/v1/login", "登录", "token"],
        ["POST", "/v1/lots", "创建批次（开工）", "批次，含 start_time"],
        ["PUT", "/v1/lots/:id/status", "批次状态流转", "批次，含 end_time"],
        ["POST", "/v1/doffing", "创建落筒记录", "落筒记录（pending）"],
        ["GET", "/v1/doffing", "落筒列表", "落筒列表 + pagination"],
        ["GET", "/v1/doffing/:id", "落筒详情", "单条落筒"],
        ["PUT", "/v1/doffing/:id/confirm", "确认落筒", "落筒记录（confirmed，含重量与丝饼号）"],
        ["PUT", "/v1/doffing/:id/cancel", "作废落筒", "落筒记录（cancelled）"],
        ["GET", "/v1/bobbins", "丝饼列表", "丝饼列表 + pagination"],
        ["GET", "/v1/bobbins/:id", "丝饼详情", "单个丝饼"],
        ["PUT", "/v1/bobbins/:id/weigh", "称重", "丝饼，含毛重/皮重/净重"],
        ["PUT", "/v1/bobbins/:id/inspect", "质检", "质检记录，含五维分与质检项"],
        ["PUT", "/v1/bobbins/:id/status", "丝饼状态更新", "丝饼"],
        ["POST", "/v1/bobbins/:id/print", "标记标签已打印", "丝饼"],
        ["POST", "/v1/pallets", "创建托盘", "托盘（building）"],
        ["POST", "/v1/pallets/:id/pack", "丝饼码盘（POY/FDY）", "托盘"],
        ["POST", "/v1/pallets/:id/pack-cartons", "纸箱码垛（DTY）", "托盘"],
        ["POST", "/v1/pallets/:id/seal", "封托", "托盘（completed，含三层重量）"],
        ["POST", "/v1/barrels", "创建落纱桶", "桶"],
        ["POST", "/v1/barrels/:id/load", "按锭位装桶", "桶，含装载量"],
        ["POST", "/v1/barrels/:id/seal", "封桶", "桶（full）"],
        ["POST", "/v1/cartons", "创建纸箱", "纸箱（packing）"],
        ["POST", "/v1/cartons/:id/pack", "丝饼装箱", "纸箱"],
        ["POST", "/v1/cartons/:id/seal", "封箱", "纸箱（packed）"],
        ["POST", "/v1/modules", "创建吊车（丝车）", "吊车"],
        ["POST", "/v1/modules/:id/load", "装桶", "吊车，含两个桶 ID"],
        ["POST", "/v1/modules/:id/status", "更新车辆状态与位置", "吊车"],
        ["POST", "/v1/sync/upload", "上传数据到 Center", "applied / rejected / new_cursor"],
        ["POST", "/v1/sync/download", "拉取基础数据", "线体、等级字典"],
        ["POST", "/v1/sync/resync", "重置同步状态", "—"],
    ]
    ws = wb.create_sheet("接口清单-Edge")
    ws["A1"] = "Edge 边端对外接口（现场业务写入口；Center 只读）"
    ws["A1"].font = TITLE_FONT
    write_header(ws, 3, headers, widths)
    write_rows(ws, 4, edge_rows)
    ws.freeze_panes = "A4"

    # ============ 页面功能 + 分角色截图 ============
    ws = wb.create_sheet("页面功能说明")
    ws["A1"] = "页面功能说明（含各角色实拍截图）"
    ws["A1"].font = TITLE_FONT
    headers = ["序号", "页面", "区域", "功能说明", "对应接口"]
    widths = [6, 20, 24, 88, 34]
    write_header(ws, 3, headers, widths)
    funcs = [
        [1, "登录页", "用户名 / 密码", "账号密码登录，签发 JWT（默认 2 小时）；401 自动回登录页", "POST /v1/login"],
        [2, "批次管理", "筛选栏", "状态、产品类型、订单号（精确）、批次号（模糊）", "GET /v1/lots"],
        [3, "批次管理", "列表", "批次号、产品+规格、订单号、计划/实际数量、进度条、状态、来源设备、创建时间", "GET /v1/lots"],
        [4, "批次详情", "基本信息", "PLC 批次号、订单号、产品类型/规格、计划/实际数量、进度、设备、时间", "GET /v1/lots/:id"],
        [5, "批次详情", "丝饼列表", "分页 + 编码筛选；位置、等级、净重、状态、落筒时间", "GET /v1/bobbins?lot_id="],
        [6, "批次详情", "质检记录", "四档筛选；综合得分、五维评分、质检项三态、缺陷原因、质检员", "GET /v1/grades?lot_id="],
        [7, "批次详情", "追溯链", "批次→落筒→丝饼→质检→包装（桶/托盘/纸箱实时计数）", "GET /v1/doffings、/v1/barrels、/v1/pallets、/v1/cartons"],
        [8, "托盘管理", "列表 + 详情", "类型（码锭/码箱）、层级、装载数、三层重量、状态、封托时间；详情含盘内丝饼或纸箱", "GET /v1/pallets"],
        [9, "落纱桶", "列表 + 详情", "装载进度环、状态、装满时间；详情含桶内丝饼与实际/记账枚数对照", "GET /v1/barrels"],
        [10, "纸箱", "列表 + 详情", "装箱数/容量、总重量、所属托盘、封箱时间；详情含箱内丝饼", "GET /v1/cartons"],
        [11, "吊车", "列表", "车编号、挂位 1/2、状态、当前位置、RFID", "GET /v1/modules"],
        [12, "报表", "区间 + 生产概览", "批次数、计划/实际产量、完成率；按状态、产品类型、来源设备分组", "GET /v1/reports/production-overview"],
        [13, "报表", "质量分布", "合格率（分母不含未判定）、平均分、评分样本、等级分布、缺陷 Top N、质检项合格率", "GET /v1/reports/quality-distribution"],
        [14, "报表", "机台与操作员", "按机台产量、按操作员落筒数、按检测员工检数", "GET /v1/reports/machine-operator"],
        [15, "系统管理-用户", "列表 + 行操作", "新建、编辑、改密、重置密码、停用/启用、删除（仅管理员可见）", "GET/POST/PUT/DELETE /v1/users"],
        [16, "系统管理-设备", "列表 + 行操作", "注册设备、详情、签发设备凭证（仅管理员可见）", "GET/POST /v1/edges、POST /v1/edges/:code/token"],
    ]
    r = write_rows(ws, 4, funcs)
    ws.freeze_panes = "A4"

    # 截图分块：按角色
    blocks = [
        ("管理员（admin）：菜单含系统管理", "admin", [
            ("00-home.png", "批次管理（首页）"),
            ("02-lot-detail.png", "批次详情（含追溯链、包装计数）"),
            ("03-pallets.png", "托盘管理"),
            ("04-barrels.png", "落纱桶"),
            ("05-cartons.png", "纸箱"),
            ("06-modules.png", "吊车"),
            ("07-reports.png", "报表"),
            ("08-users.png", "系统管理 → 用户"),
            ("09-edges.png", "系统管理 → 设备"),
            ("10-pallet-detail.png", "托盘详情"),
            ("11-barrel-detail.png", "落纱桶详情"),
            ("12-carton-detail.png", "纸箱详情"),
        ]),
        ("操作员（operator1）：无系统管理菜单", "operator", [
            ("00-home.png", "批次管理"),
            ("02-lot-detail.png", "批次详情"),
            ("07-reports.png", "报表"),
        ]),
        ("质检员（inspector1）：无系统管理菜单", "inspector", [
            ("00-home.png", "批次管理"),
            ("02-lot-detail.png", "批次详情（质检记录在此查看）"),
            ("07-reports.png", "报表"),
        ]),
        ("只读用户（viewer1）：无系统管理菜单", "viewer", [
            ("00-home.png", "批次管理"),
            ("02-lot-detail.png", "批次详情"),
            ("03-pallets.png", "托盘管理（只读）"),
        ]),
    ]

    r += 2
    for title, role, shots in blocks:
        ws.cell(row=r, column=1, value=title).font = TITLE_FONT
        r += 1
        for fname, label in shots:
            path = os.path.join(SHOTS, f"role-{role}-{fname}")
            ws.cell(row=r, column=1, value=label).font = Font(bold=True)
            r += 1
            if not os.path.exists(path):
                ws.cell(row=r, column=1, value=f"（缺少截图 {path}）")
                r += 1
                continue
            img = XLImage(path)
            if img.width > 940:
                ratio = 940 / img.width
                img.width = 940
                img.height = int(img.height * ratio)
            ws.add_image(img, f"A{r}")
            r += int(img.height / 20) + 2

    # ============ 层级核对 ============
    headers = ["核对项", "现状（已核实）", "需客户确认", "建议方案"]
    widths = [20, 64, 54, 50]
    rows = [
        ["一次落筒的丝饼数",
         "现模型一次 doffing = 1 个锭位 1 枚丝饼，无「一次落筒 = 12 枚」的分组实体",
         "是否新增落筒分组（DoffingGroup），还是在对接层按「线号 + 位号 + 落筒时间窗」聚合出 12 条",
         "对接层先聚合（改动小）；若客户要求落筒级回执，再加分组实体"],
        ["丝车实体",
         "无 trolley 表。最接近的是吊车（挂 2 桶）与落纱桶（容量 1–9）；整车称重无接口",
         "丝车是否等同「吊车 + 桶」结构；是否需要整车称重与 Empty/Full/Remain 状态",
         "新增丝车实体（trolley）+ 整车称重接口；现有的 module 继续承担车间转运"],
        ["数量整除关系",
         "12×2=24、12×3=36；托盘 48 枚（演示数据：POY 24 枚/托、FDY 24 枚/托、DTY 4 箱=48 枚）",
         "FDY 的 36 枚如何凑 48 枚/托：补车、混批，还是 FDY 另设 36 枚规格",
         "按产品类型分规格：POY 24/托、FDY 36/托、DTY 48/托（4 箱×12）"],
        ["托盘明细的关联深度",
         "托盘接口只带 bobbins_count / cartons_count，明细需另查 /v1/bobbins?pallet_id=",
         "出货报文是否需要内嵌 32/48 条丝饼明细（单包体积约 6KB）",
         "提供「带明细」的可选参数，默认不带，避免常规查询变重"],
        ["时间口径",
         "接口返回 UTC（RFC3339），前端按本地时区展示；客户 ERP 侧为本地时间",
         "对外报文用 UTC 还是本地时间（含时区偏移）",
         "对外报文统一 ISO8601 带偏移，如 2026-11-15T09:41:07+08:00"],
    ]
    ws = wb.create_sheet("层级核对")
    ws["A1"] = "三层包含关系与待确认项（2026-11-15 核实）"
    ws["A1"].font = TITLE_FONT
    write_header(ws, 3, headers, widths)
    write_rows(ws, 4, rows)
    ws.freeze_panes = "A4"

    ws.cell(row=4 + len(rows) + 2, column=1,
            value="包含关系：批次 → 落筒（一次 12 枚）→ 丝饼 → 丝车（2/3 个落筒）→ 托盘（48 枚，不带丝车/落筒号）")
    ws.cell(row=4 + len(rows) + 2, column=1).font = Font(bold=True)

    # ============ 已知问题与处理 ============
    headers = ["#", "问题", "现象", "根因", "处理结果"]
    widths = [5, 30, 46, 52, 40]
    rows = [
        [1, "托盘接口 500", "GET /v1/pallets 返回 column pallets.pallet_type does not exist",
         "开发库表结构停在上个版本，缺 pallet_type / cartons_count；cartons 缺 capacity / pallet_id",
         "已执行 Schema.Create 迁移，接口恢复正常（本次核实 11 条托盘可查）"],
        [2, "设备名称乱码", "edge-e2e-pallet 设备名显示为方块",
         "测试脚本以非 UTF-8 编码写入",
         "已改为「二号边端（端到端验证）」，页面显示正常"],
        [3, "追溯链包装层文案过期", "批次详情写「Center 暂无这些表的数据」",
         "包装四表接通后文案未同步",
         "已改为实时计数：落纱桶 / 托盘 / 纸箱各自数量"],
        [4, "非管理员可见系统管理菜单", "查看员登录后能看到系统管理，点进去报「无权访问该接口」",
         "菜单项未按角色过滤",
         "已按角色隐藏：非 admin 角色只显示 6 个业务菜单"],
        [5, "演示数据缺失", "此前页面大量空表，无法说明功能",
         "无演示数据",
         "已生成 3 个批次（DTY/POY/FDY）、19 次落筒、228 枚丝饼、228 条质检、22 个桶、11 台吊车、5 个纸箱、11 个托盘"],
    ]
    ws = wb.create_sheet("已知问题与处理")
    ws["A1"] = "本轮核实发现的问题与处理结果"
    ws["A1"].font = TITLE_FONT
    write_header(ws, 3, headers, widths)
    write_rows(ws, 4, rows)
    ws.freeze_panes = "A4"

    os.makedirs(os.path.join("docs", "delivery"), exist_ok=True)
    target = resolve_out(OUT)
    wb.save(target)
    print("saved:", target)


if __name__ == "__main__":
    main()
