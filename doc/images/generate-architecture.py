#!/usr/bin/env python3
"""Generate the README architecture SVGs using only the Python standard library.

Run: python3 doc/images/generate-architecture.py
Scope: ordinary amd64/arm64 GoObj compilation. Pipeline references:
  doc/goallc-llvm-pipeline.md
  src/cmd/llvmplugin/README.md
"""
from html import escape
from pathlib import Path

OUT = Path(__file__).resolve().parent
COLORS = {
    'go': ('#EEF2F6', '#64748B', '#1E293B'),
    'goallc': ('#175CD3', '#1249A6', '#FFFFFF'),
    'llvm': ('#0E766E', '#09594F', '#FFFFFF'),
    'extension': ('#FDB022', '#DC6803', '#422006'),
    'data': ('#FFFFFF', '#94A3B8', '#1E293B'),
}
TEXT = {
    'en': {
        'title': 'GoALLC architecture',
        'subtitle': 'Ordinary GoObj compilation · amd64 / arm64 · LLVM runs inside the compiler',
        'legend': ['Reused Go', 'GoALLC implementation', 'Standard LLVM', 'LLVM fork extensions'],
        'front': ['cmd/go · Go frontend · generic SSA', 'Parsing, types, inlining, escape analysis and Go optimizations'],
        'functions': ['ssa2llvm / llvmfunctions / llvmdebug', 'Functions · Go ABI · closures · GC / debug metadata'],
        'globals': ['llvmdata / llvmtypeddata', 'Types · itabs · globals · symbols / relocations'],
        'module': ['Package LLVM Module', 'Function IR + global IR + Go metadata · go-llvm C/C++ API bridge'],
        'group': 'LLVM optimization and code generation',
        'left': 'Compilation stages', 'right': 'Go-specific passes and target support',
        'opt': ['LLVM IR optimizer', 'Loop / SLP vectorization · inlining · scalar optimization'],
        'early': ['GoALLC early IR preparation', 'CPU features / FMV · write-barrier contracts'],
        'hook': ['llvmbackend · pre-codegen hook', 'Prepare Go semantics and register target passes'],
        'wb': ['GoALLCWriteBarriers / GoALLCByVal', 'Late barrier lowering · byval initialization'],
        'preisel': ['CodeGen IR prep → GoALLCPreISelPass', 'Late statepoint rewrite, just before instruction selection'],
        'sp': ['GoALLCStatepoints', 'Pointer liveness · gc.relocate · fixed-frame objects'],
        'isel': ['SelectionDAG → Machine IR', 'Call lowering · instruction selection · statepoint lowering'],
        'abi': ['X86 / AArch64 · Go ABI', 'ABIInternal / ABI0 · registers · stack conventions'],
        'machine': ['Machine passes / Go frame lowering', 'Scheduling · register allocation · Go stack checks'],
        'preempt': ['GoALLCAsyncPreemption', 'Final machine instructions → unsafe PC ranges'],
        'writer': ['AsmPrinter → GoObj writer', 'MCGoObjStreamer / GoObjObjectWriter', 'Machine code · Go symbols · relocations'],
        'metadata': ['StackMaps / GoObjDebug / PC tables', 'Pointer maps · stack objects · PCDATA / FUNCDATA', 'PCSP · file / line · inline tree · DWARF'],
        'end': ['Go archive → cmd/link → executable + Go runtime', '__.PKGDEF + _go_.o + assembly / cgo objects', 'Existing Go linking, GC, stack copying, reflection and diagnostics'],
        'before': 'before', 'calls': 'calls', 'target': 'ABI', 'pc': 'PC ranges',
        'serialize': 'serialize', 'exports': 'Go package export data',
        'flow': 'Compilation / artifacts', 'meta': 'Metadata / integration',
    },
    'zh-CN': {
        'title': 'GoALLC 架构',
        'subtitle': '普通 GoObj 编译路径 · amd64 / arm64 · LLVM 在编译器进程内运行',
        'legend': ['复用 Go', 'GoALLC 实现', 'LLVM 标准模块', 'LLVM fork 扩展'],
        'front': ['cmd/go · Go 前端 · 通用 SSA', '语法与类型检查、内联、逃逸分析及 Go 通用优化'],
        'functions': ['ssa2llvm / llvmfunctions / llvmdebug', '函数 · Go ABI · 闭包 · GC / 调试元数据'],
        'globals': ['llvmdata / llvmtypeddata', '类型 · itab · 全局数据 · 符号 / 重定位'],
        'module': ['包级 LLVM Module', '函数 IR + 全局 IR + Go 元数据 · 通过 go-llvm C/C++ API 桥接'],
        'group': 'LLVM 优化与代码生成',
        'left': '编译阶段', 'right': 'Go 专用 pass 与目标支持',
        'opt': ['LLVM IR optimizer', '循环 / SLP 向量化 · 内联 · 标量优化'],
        'early': ['GoALLC early IR preparation', 'CPU features / FMV · 写屏障契约'],
        'hook': ['llvmbackend · pre-codegen hook', '处理 Go 语义并注册目标 pass'],
        'wb': ['GoALLCWriteBarriers / GoALLCByVal', '写屏障晚期展开 · byval 初始化'],
        'preisel': ['CodeGen IR prep → GoALLCPreISelPass', '紧邻指令选择之前，执行晚期 statepoint 重写'],
        'sp': ['GoALLCStatepoints', '指针活跃性 · gc.relocate · 固定栈对象'],
        'isel': ['SelectionDAG → Machine IR', '调用 lowering · 指令选择 · statepoint lowering'],
        'abi': ['X86 / AArch64 · Go ABI', 'ABIInternal / ABI0 · 寄存器 · 栈约定'],
        'machine': ['Machine passes / Go frame lowering', '指令调度 · 寄存器分配 · Go 栈检查'],
        'preempt': ['GoALLCAsyncPreemption', '最终机器指令 → 不安全抢占 PC 区间'],
        'writer': ['AsmPrinter → GoObj writer', 'MCGoObjStreamer / GoObjObjectWriter', '机器码 · Go 符号 · 重定位'],
        'metadata': ['StackMaps / GoObjDebug / PC tables', '指针位图 · 栈对象 · PCDATA / FUNCDATA', 'PCSP · 文件与行号 · 内联树 · DWARF'],
        'end': ['Go 包归档 → cmd/link → 可执行文件 + Go runtime', '__.PKGDEF + _go_.o + 汇编 / cgo 对象', '复用 Go 链接流程、GC、栈复制、反射与诊断工具'],
        'before': '之前', 'calls': '调用', 'target': 'ABI', 'pc': 'PC 区间',
        'serialize': '序列化', 'exports': 'Go 包导出数据',
        'flow': '编译流程 / 产物流', 'meta': '元数据 / 集成关系',
    },
}


def generate(lang):
    t = TEXT[lang]
    parts = [
        '<svg xmlns="http://www.w3.org/2000/svg" width="1260" height="1660" viewBox="0 0 1260 1660" role="img" aria-labelledby="title description">',
        f'<title id="title">{escape(t["title"])}</title>',
        f'<desc id="description">{escape(t["subtitle"])}</desc>',
        '<defs><marker id="arrow" markerWidth="9" markerHeight="9" refX="8" refY="4.5" orient="auto" markerUnits="userSpaceOnUse"><path d="M0,0 L9,4.5 L0,9 Z" fill="#475569"/></marker></defs>',
        '<rect width="1260" height="1660" rx="16" fill="#FFFFFF"/>',
        '<g font-family="Arial, PingFang SC, Microsoft YaHei, Noto Sans CJK SC, sans-serif">',
    ]

    def text(x, y, value, size=22, fill='#1E293B', bold=False, anchor='start', extra=''):
        parts.append(f'<text x="{x}" y="{y}" font-size="{size}" fill="{fill}" font-weight="{700 if bold else 400}" text-anchor="{anchor}" {extra}>{escape(value)}</text>')

    def box(key, x, y, w, h, kind):
        bg, stroke, fg = COLORS[kind]
        parts.append(f'<g data-node="{key}"><rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{bg}" stroke="{stroke}" stroke-width="2"/>')
        lines = t[key]
        first = y + h / 2 - (len(lines) - 1) * 16 + 8
        for i, line in enumerate(lines):
            text(x+w/2, first+i*32, line, 23 if i == 0 else 19,
                 fg, i == 0, 'middle')
        parts.append('</g>')

    def edge(points, dashed=False, label=None, at=None):
        dash = ' stroke-dasharray="6 6"' if dashed else ''
        points = ' '.join(f'{x},{y}' for x, y in points)
        parts.append(f'<polyline points="{points}" fill="none" stroke="#475569" stroke-width="2"{dash} marker-end="url(#arrow)"/>')
        if label:
            text(*at, t[label], 15, '#475569', anchor='middle',
                 extra='stroke="white" stroke-width="6" paint-order="stroke"')

    text(50, 52, t['title'], 36, bold=True)
    text(50, 89, t['subtitle'], 20, '#475569')
    for x, kind, label in zip([50, 300, 650, 950], ['go', 'goallc', 'llvm', 'extension'], t['legend']):
        bg, stroke, _ = COLORS[kind]
        parts.append(f'<rect x="{x}" y="114" width="22" height="22" rx="4" fill="{bg}" stroke="{stroke}"/>')
        text(x+32, 132, label, 19)

    box('front', 50, 166, 1150, 92, 'go')
    box('functions', 50, 310, 560, 92, 'goallc')
    box('globals', 720, 310, 480, 92, 'goallc')
    box('module', 50, 454, 1150, 88, 'data')
    edge([(330, 258), (330, 310)])
    edge([(960, 258), (960, 310)])
    edge([(330, 402), (330, 454)])
    edge([(960, 402), (960, 454)])
    # Package export data bypasses LLVM and joins the Go archive below.
    edge([(50, 211), (22, 211), (22, 1514), (50, 1514)], True)
    text(15, 925, t['exports'], 16, '#64748B', anchor='middle',
         extra='transform="rotate(-90 15 925)"')

    parts.append('<rect x="40" y="574" width="1170" height="848" rx="14" fill="#F8FAFC" stroke="#CBD5E1"/>')
    text(60, 608, t['group'], 25, bold=True)
    text(330, 642, t['left'], 17, '#475569', anchor='middle')
    text(960, 642, t['right'], 17, '#475569', anchor='middle')
    rows = [
        (662, 'opt', 'llvm', 'early', 'goallc', 'before', True),
        (784, 'hook', 'goallc', 'wb', 'goallc', 'calls', False),
        (906, 'preisel', 'goallc', 'sp', 'goallc', 'calls', False),
        (1028, 'isel', 'llvm', 'abi', 'extension', 'target', True),
        (1150, 'machine', 'extension', 'preempt', 'goallc', 'calls', False),
        (1272, 'writer', 'extension', 'metadata', 'extension', 'serialize', True),
    ]
    edge([(665, 542), (665, 652), (330, 652), (330, 662)])
    for i, (y, left, lk, right, rk, label, to_left) in enumerate(rows):
        h = 116 if left == 'writer' else 88
        box(left, 50, y, 560, h, lk)
        box(right, 720, y, 480, h, rk)
        start, end = ((720, y+h/2), (610, y+h/2)) if to_left else ((610, y+h/2), (720, y+h/2))
        edge([start, end], True, label, (665, y+h/2-12))
        if i:
            edge([(330, rows[i-1][0]+88), (330, y)])
    edge([(960, 1238), (960, 1272)], True)
    edge([(1200, 950), (1230, 950), (1230, 1330), (1200, 1330)], True)
    box('end', 50, 1470, 1150, 116, 'go')
    edge([(330, 1388), (330, 1470)])
    edge([(60, 1625), (115, 1625)])
    text(130, 1632, t['flow'], 18, '#475569')
    edge([(650, 1625), (705, 1625)], True)
    text(720, 1632, t['meta'], 18, '#475569')
    parts.append('</g></svg>')
    (OUT / f'goallc-architecture.{lang}.svg').write_text('\n'.join(parts)+'\n')


if __name__ == '__main__':
    for language in TEXT:
        generate(language)
