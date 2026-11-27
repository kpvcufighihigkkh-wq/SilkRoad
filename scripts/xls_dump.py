"""最小 BIFF8 (.xls) 读取器：仅供一次性读取客户参考表，不进产品代码。

用法：python scripts/xls_dump.py <file.xls> [max_rows]
只支持 BIFF8 的 SST / LABELSST / NUMBER / RK / MULRK / LABEL 六种记录，
足够 dump 出对接表的单元格文本。
"""

import struct
import sys


def read_ole_streams(data):
    """解析 OLE2 复合文档，返回 {stream_name: bytes}。"""
    ss = 1 << struct.unpack_from("<H", data, 30)[0]
    nfat = struct.unpack_from("<I", data, 44)[0]
    dirstart = struct.unpack_from("<I", data, 48)[0]
    mini_fat_start = struct.unpack_from("<I", data, 60)[0]
    ndif = struct.unpack_from("<I", data, 72)[0]
    difstart = struct.unpack_from("<I", data, 68)[0]

    def sect_off(s):
        return 512 + s * ss

    fat_sects = list(struct.unpack_from("<109I", data, 76))
    s, cnt = difstart, 0
    while s not in (0xFFFFFFFE, 0xFFFFFFFF) and cnt < ndif:
        vals = struct.unpack_from("<%dI" % (ss // 4), data, sect_off(s))
        fat_sects.extend(vals[:-1])
        s = vals[-1]
        cnt += 1

    fat = []
    for fs in fat_sects[:nfat]:
        if fs >= 0xFFFFFFFC:
            continue
        fat.extend(struct.unpack_from("<%dI" % (ss // 4), data, sect_off(fs)))

    def chain(start):
        out, s = [], start
        while s not in (0xFFFFFFFE, 0xFFFFFFFF) and s < len(fat):
            out.append(s)
            s = fat[s]
            if len(out) > 200000:
                break
        return out

    def read_chain(start, size=None):
        blob = b"".join(data[sect_off(x):sect_off(x) + ss] for x in chain(start))
        return blob[:size] if size else blob

    dirdata = read_chain(dirstart)
    entries = []
    for i in range(0, len(dirdata), 128):
        e = dirdata[i:i + 128]
        if len(e) < 128:
            break
        nlen = struct.unpack_from("<H", e, 64)[0]
        name = e[:max(0, nlen - 2)].decode("utf-16-le", "ignore")
        typ = e[66]
        start = struct.unpack_from("<I", e, 116)[0]
        size = struct.unpack_from("<I", e, 120)[0]
        entries.append((name, typ, start, size))

    root = entries[0]
    minidata = read_chain(root[2]) if root[2] < 0xFFFFFFFC else b""
    minifat = []
    for s in chain(mini_fat_start):
        minifat.extend(struct.unpack_from("<%dI" % (ss // 4), data, sect_off(s)))

    def read_mini(start, size):
        out, s = b"", start
        while s not in (0xFFFFFFFE, 0xFFFFFFFF) and s < len(minifat):
            out += minidata[s * 64:s * 64 + 64]
            s = minifat[s]
        return out[:size]

    streams = {}
    for name, typ, start, size in entries:
        if typ != 2:
            continue
        streams[name] = read_mini(start, size) if size < 4096 else read_chain(start, size)
    return streams


def records(wb):
    i = 0
    while i + 4 <= len(wb):
        rtype, rlen = struct.unpack_from("<HH", wb, i)
        yield i, rtype, wb[i + 4:i + 4 + rlen]
        i += 4 + rlen


def parse_sst(recs, idx):
    """解析 SST（可能被 CONTINUE 分段），返回字符串表。"""
    segs = [recs[idx][2]]
    j = idx + 1
    while j < len(recs) and recs[j][1] == 0x003C:
        segs.append(recs[j][2])
        j += 1

    _total, unique = struct.unpack_from("<II", segs[0], 0)
    cur, pos = 0, 8
    out = []
    for _ in range(unique):
        while pos >= len(segs[cur]) and cur + 1 < len(segs):
            cur, pos = cur + 1, 0
        if pos + 3 > len(segs[cur]):
            cur, pos = cur + 1, 0
        cch = struct.unpack_from("<H", segs[cur], pos)[0]
        pos += 2
        flags = segs[cur][pos]
        pos += 1
        is16 = bool(flags & 1)
        rich, ext = bool(flags & 8), bool(flags & 4)
        nrich = cbext = 0
        if rich:
            nrich = struct.unpack_from("<H", segs[cur], pos)[0]
            pos += 2
        if ext:
            cbext = struct.unpack_from("<I", segs[cur], pos)[0]
            pos += 4

        need, chars = cch, []
        while need > 0:
            avail = (len(segs[cur]) - pos) // 2 if is16 else (len(segs[cur]) - pos)
            take = min(need, avail)
            if is16:
                chars.append(segs[cur][pos:pos + take * 2].decode("utf-16-le", "ignore"))
                pos += take * 2
            else:
                chars.append(segs[cur][pos:pos + take].decode("latin-1"))
                pos += take
            need -= take
            if need > 0:
                cur, pos = cur + 1, 1
                is16 = bool(segs[cur][0] & 1)
        if rich:
            pos += nrich * 4
        if ext:
            pos += cbext
        out.append("".join(chars))
    return out


def dump(path, max_rows=60):
    data = open(path, "rb").read()
    wb = read_ole_streams(data)["Workbook"]
    recs = list(records(wb))

    bounds = [(off, b) for off, t, b in recs if t == 0x85]
    starts = [struct.unpack_from("<I", b, 0)[0] for _, b in bounds]
    names = []
    for _, b in bounds:
        cch, flags = b[6], b[7]
        names.append(b[8:8 + cch * 2].decode("utf-16-le", "ignore") if flags & 1
                     else b[8:8 + cch].decode("latin-1"))

    sst_idx = next((k for k, (_, t, _) in enumerate(recs) if t == 0x00FC), None)
    sst = parse_sst(recs, sst_idx) if sst_idx is not None else []

    for k, name in enumerate(names):
        s = starts[k]
        e = starts[k + 1] if k + 1 < len(starts) else len(wb)
        rows = {}
        for off, t, b in recs:
            if not (s <= off < e):
                continue
            if t == 0x00FD:  # LABELSST
                r, c, _xf, sidx = struct.unpack_from("<HHHI", b, 0)
                rows.setdefault(r, {})[c] = sst[sidx] if sidx < len(sst) else ""
            elif t == 0x0203:  # NUMBER
                r, c, _xf = struct.unpack_from("<HHH", b, 0)
                rows.setdefault(r, {})[c] = struct.unpack_from("<d", b, 6)[0]
            elif t == 0x027E:  # RK
                r, c, _xf = struct.unpack_from("<HHH", b, 0)
                rk = struct.unpack_from("<i", b, 6)[0]
                rows.setdefault(r, {})[c] = (rk >> 2) if rk & 2 else rk / 100.0
            elif t == 0x00BD:  # MULRK
                r, c1 = struct.unpack_from("<HH", b, 0)
                for j in range((len(b) - 6) // 6):
                    rk = struct.unpack_from("<i", b, 4 + j * 6 + 2)[0]
                    rows.setdefault(r, {})[c1 + j] = (rk >> 2) if rk & 2 else rk / 100.0
            elif t == 0x0204:  # LABEL
                r, c, _xf, cch = struct.unpack_from("<HHHH", b, 0)
                flags = b[8]
                rows.setdefault(r, {})[c] = (b[9:9 + cch * 2].decode("utf-16-le", "ignore")
                                            if flags & 1 else b[9:9 + cch].decode("latin-1"))

        print("=" * 90)
        print("SHEET", name)
        for r in sorted(rows)[:max_rows]:
            col = max(rows[r]) if rows[r] else -1
            line = " | ".join(str(rows[r].get(c, "")) for c in range(col + 1))
            if line.strip(" |"):
                print(f"[{r + 1}] {line[:800]}")


if __name__ == "__main__":
    dump(sys.argv[1], int(sys.argv[2]) if len(sys.argv) > 2 else 60)
