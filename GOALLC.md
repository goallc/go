# GoALLC 架构与实现边界

更新日期：2026-11-02。本文描述 `go1.27.master` 的架构；具体版本、平台资格和
测试结果以所用 Go revision、LLVM payload manifest 和对应测试记录为准。
早期 sidecar/toolexec 原型及升级记录保留在 Git 历史中，不作为当前使用说明。

## 项目目标与上游 Go 的关系

GoALLC 在 Go 工具链中接入 LLVM 后端，同时复用 Go 的语言前端、通用 SSA、
运行时、标准库、包归档和链接协议。正常的非 bootstrap compiler 默认使用
LLVM；`-enablellvm=false` 显式选择原生 Go 后端。默认选择不代表所有平台、
构建模式和测试已经达到上游 Go 的兼容性范围。

| 层次 | 上游 Go | GoALLC LLVM 路径 |
| --- | --- | --- |
| 语言前端 | parser、类型检查、Go IR、内联、逃逸分析 | 复用 Go 前端 |
| 通用优化 | Go generic SSA passes | 保留共享 pass 顺序，用 `nativeOnly` / `llvmOnly` 选择边界 |
| 目标后端 | Go target lowering、调度、寄存器分配、栈帧和指令生成 | Go SSA → LLVM IR；LLVM 优化、CodeGen 和 GoALLC 插件 |
| 对象输出 | 原生 Go object writer | LLVM GoObj writer |
| 包与链接 | Go export data、archive、Go linker | 普通编译继续使用这些协议 |
| 运行时 | GC、goroutine、栈增长、panic/defer、反射 | 通过 Go ABI 和元数据接入 Go runtime |

LLVM IR 交接点在 `writebarrier`、`cpufeatures` 及 LLVM 边界清理之后，
`rewrite tern` 和 target `lower` 之前。逻辑参数和结果尚未经过原生
`expand calls`。`LLVMCompile` 结束后退出 SSA pass 循环，`ssagen` 不执行
原生 `AllocFrame`；LLVM 拥有指令选择、寄存器分配和物理栈帧。Go 的 ABI
分析、ArgInfo 和 package-level data 准备仍执行。

实现入口：[`compile.go`](src/cmd/compile/internal/ssa/compile.go)、
[`ssagen/ssa.go`](src/cmd/compile/internal/ssagen/ssa.go)。具体 pass 选择见
[Go SSA before LLVM](doc/goallc-llvm-pipeline.md)。

## 仓库职责

| 仓库 / 目录 | 职责 |
| --- | --- |
| [goallc/go](https://github.com/goallc/go) | Go 工具链、SSA lowering、函数与类型模型、debug、runtime 集成和回归测试 |
| `src/cmd/llvmplugin`（Go 仓库内的 C++ 工程） | Early IR / CPU FMV、写屏障晚期分析、statepoint rewrite、机器码抢占范围分析 |
| [goallc/go-llvm](https://github.com/goallc/go-llvm) | LLVM API binding、GoObj 配置、进程内 plugin/codegen hook；vendor 到 `src/cmd/vendor` |
| [goallc/llvm-project](https://github.com/goallc/llvm-project) | 定制 LLVM 23：Go ABI、Go IR 扩展、X86/AArch64、GoObj 和调试元数据；提供插件扩展点 |

Go 专用分析策略位于 Go 仓库插件中，LLVM 负责目标机制与对象格式。插件不安装
在 LLVM payload 内。动态与静态构建使用同一套实现，区别是加载动态模块还是
直接调用链接进 compiler 的实现。

## 普通 package 编译链路

```text
go build / go test
  -> cmd/compile：Go 前端与通用 SSA
  -> 内存中的 LLVM Module（函数、数据、Go 契约、debug graph）
  -> GoALLC early IR：CPU/FMV 准备、write-barrier record 属性
  -> LLVM IR 优化：默认 O2；-N 对应 O0
  -> pre-codegen callback：写屏障展开、模块准备、注册 target passes
  -> 标准 LLVM CodeGen IR 准备
  -> late PreISel：逐函数 statepoint / relocation SSA 重写
  -> LLVM 指令选择、寄存器分配、Go 栈帧、扩栈与机器布局
  -> 最终机器位置、PC、StackMaps、debug -> GoObj
  -> Go archive writer：__.PKGDEF + 唯一 _go_.o
  -> Go linker -> 可执行程序
```

普通编译不启动外部 `opt`、`llc` 或 `toolexec`，也不依赖文本 IR 中转。
`-llvm-keep-ir` 只增加诊断文件：`.raw.ll` 是前端快照，`.ll` 是 early IR 准备后的
输入，`.opt.ll` 是 IR 优化结果；后者不是 late statepoint 重写后的最终 IR。
`__.PKGDEF` 由 Go compiler 生成。包的汇编等其他成员仍按构建工具的正常流程
处理；普通 LLVM package 不另合并一份原生 compiler 数据对象。

实现入口：[`llvmbackend.go`](src/cmd/compile/internal/ssa/llvmbackend.go)、
[`gc/obj.go`](src/cmd/compile/internal/gc/obj.go)、
[`GoALLCStatepointPlugin.cpp`](src/cmd/llvmplugin/GoALLCStatepointPlugin.cpp)。

## LLVM 侧的 Go 能力

| 能力 | 主要契约 |
| --- | --- |
| 调用约定 | `goabiinternal` / `goabi0`、完整值的参数/结果分类、参数 homes、typed `byval` / `goret`、隐藏 `nest` context |
| 栈与目标 ABI | X86/AArch64 Go frame、默认 split-stack 检查、`morestack` 重试、`go-nosplit` / `go-systemstack`、保留寄存器状态 |
| Go IR 语义 | `llvm.go.defer.edge`、`llvm.go.abi0.frame`、pointer address/from-address、私有 ABI 写屏障 buffer intrinsic |
| GC 元数据 | 标准 statepoint 机器位置到 Args/LocalsPointerMaps、StackObjects、PCDATA_StackMapIndex 的适配 |
| 异步抢占 | Go 插件根据最终机器指令标记 unsafe ranges，LLVM 序列化标签到 PC 的范围 |
| GoObj | Go header、包/符号身份、ABI、content hash、relocations、aux、类型与链接器 marker |
| 可观测性 | PCSP、文件/行号、内联树、ArgInfo/ArgLive、Go linker-native DWARF carriers 和变量位置 |

这些能力互相依赖：闭包的 context 寄存器也必须在扩栈慢路径中保存；GC 图必须
使用最终物理 frame 的位置。LLVM fork 的 `GoCallingConv.h`、`IntrinsicsGo.td`、
`GoObjObjectWriter.cpp` 和两个目标的 frame lowering 是相应底层实现。

## GC、栈迁移与运行时语义

Go 当前堆不移动，但 goroutine 栈可以迁移。late statepoint pass 计算调用后活跃
值，生成 `gc.statepoint`、`gc.result`、`gc.relocate`，通过 `PromoteMemToReg`
建立条件分支、循环和合流处的 relocation SSA。

固定 `alloca`、typed `byval` / `goret` 是存储身份，不作为普通指针生成
`gc.relocate`。同对象派生地址规范化为 base 加整数 offset，在实际使用处重建；
SelectionDAG 使用 canonical FrameIndex。它们的内容有独立 typed layout 和
活跃性分析；从中加载的普通指针仍按普通 root 处理。

Args/LocalsPointerMaps 按完整 pair 去重，由同一 StackMapIndex 选择。入口 pair 0
是 EntryArgs 与 empty locals。地址可观察的固定对象还生成 StackObjects；对象
身份不等于内容在每个调用点都活跃。ArgLiveIndex 根据真实参数-home stores
描述 traceback 可见性，不替代 GC 图。

Go SSA 决定写屏障语义；插件在 LLVM 优化后利用 MemorySSA、AA 和分配属性证明
可省略的记录，再展开 buffer 协议。`defer` 恢复边通过零指令的
`callbr @llvm.go.defer.edge` 保留；open-defer bits/slots 经最终 frame metadata
交给 runtime。详细协议见 [plugin README](src/cmd/llvmplugin/README.md)。

## 类型、接口与调试信息

Go `reflectdata` 仍生成 finalized `LSym`。数据 lowering 收集受支持的静态 data
symbols 及额外 roots 的 Gotype/relocation 闭包，已建 schema 的 runtime ABI 数据
使用 typed packed constants，其他部分保留 bytes/relocations。initializer 是地址关系的来源；GoObj
专有 flags、offset/marker relocations 和 aux 使用显式 metadata。

定义级 keep、gotype、weak-call 和 marker 关系随所属定义选择；需要随内联迁移的
函数 marker 先由 `llvm.sideeffect` 携带，优化后归属最终函数。LLVM native DI graph
记录包、函数、变量、类型和内联关系；最终机器位置生成 PC 表和 DWARF location lists。
优化后不可用的值保持 unavailable，不从源变量名称猜测位置。

具体协议见 [LLVM IR 到 GoObj](doc/goallc-llvm-goobj.md)，runtime 函数属性见
[Compiler-known runtime LLVM contracts](doc/goallc-runtime-attributes.md)。

## LTO 状态

截至 2026-11-02，主线 compiler 已有 `-llvm-lto`：执行 early IR 准备和
`lto-pre-link<O2>`，写入 `__.PKGDEF + __.LLVMBC`，把机器码生成留到链接阶段。
它要求 LLVM、默认优化 pipeline、启用优化、combined archive，且不支持 sanitizers
或 dynamic Go linking 等冲突选项。

主线 `cmd/link` 目前明确拒绝 `__.LLVMBC`，报告需要 LTO-capable linker。
端到端 `go build -lto=full`、跨包优化和单一物理 GoObj 在
[PR #347](https://github.com/goallc/go/pull/347) 中；该 PR 在上述日期仍未合并。
不能把 compiler bitcode 支持写成主线已具备完整 FullLTO，也不能将相关 LLVM
metadata 合并能力等同于 ThinLTO 端到端支持。

## 构建、验证与当前边界

先用 LLVM 仓库构建器生成完整安装 payload，再通过 Go 原有入口构建：

```sh
cd src
./make.bash -llvm-dir=/path/to/goallc-llvm-payload \
  -llvm-version=23 -llvm-link=dynamic
```

`-llvm-link=static` 选择静态模式。动态 `libLLVM` 与 plugin 安装到
`$GOROOT/pkg/tool/<host>/lib`；plugin 构建缓存和静态 archive 位于
`pkg/goallc-llvmplugin`。bootstrap compiler 使用原生后端；后续正常 compiler
默认 LLVM。完整构建、缓存和测试入口见 [构建说明](doc/goallc-build.md)。

当前 compiler 配置的目标是 Darwin/arm64、Linux/amd64 和 Linux/arm64。
动态/realigned frame、scalable pointer vectors、一般 moving-heap base/derived
推断和任意 LLVM EH/call shapes 不在现有协议范围内；具体支持形态由 lowering、
verifier 和回归测试约束。支持某项机制不代表所有组合已通过运行时资格验证。

回归复用 Go testdir、codegen FileCheck、plugin CTest 和 LLVM target/MC tests。
`all.bash` 的已知失败在 `test/llvm_known_failures.json` 中；当前失败应按具体
revision 与测试结果判断，不沿用早期白名单计数或开发机历史结果。

原生对照必须显式禁用 LLVM，例如：

```sh
go build -gcflags='all=-enablellvm=false' .
```

按包选择 LLVM 必须先设原生基线，再覆盖目标包；命令和范围说明见
[分层构建和测试](doc/goallc-llvm-goobj.md#标准库的分层构建和测试)。
