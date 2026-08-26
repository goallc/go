# GoALLC：LLVM IR 到 GoObj 的开发链路

本文记录 Go SSA 到 LLVM lowering 的端到端链路。默认路径在 `cmd/compile`
进程内完成 LLVM IR 优化、GoALLC pre-codegen pipeline 和 GoObj 生成，再复用
compiler 原有的 Go package archive writer；不会把 IR 写成文本，也不启动
`opt`、`llc` 或 `toolexec`。原外部链路继续保留用于 A/B 调试和独立工具检查。

这是一条受限的开发路径，不是社区 Go 后端的替代品。完整的项目背景、构建
基线和更早的试验记录见 [../GOALLC.md](../GOALLC.md)。

## 数据流

```text
go build -gcflags=all=-enablellvm
  -> compile 构造内存中的 LLVM Module
  -> Module::RunPasses（默认 default<O2>）
  -> 调用进程内 GoALLCStatepoints pre-codegen callback
  -> TargetMachine::EmitToMemoryBuffer
  -> compiler archive writer 写入 __.PKGDEF 和唯一的 _go_.o
  -> cmd/link 读取 __.PKGDEF 和 _go_.o
```

`-enablellvm` 现在选择进程内 LLVM backend。`__.PKGDEF` 仍完全由 Go compiler
生成并位于 archive 的第一个成员；LLVM 返回的内存 object 直接成为唯一的
`_go_.o`。`-linkobj` 拆分输出也沿用原 compiler 语义。

`-llvm-keep-ir` 只控制诊断输出：进程内 backend 仍完整生成 GoObj，同时保留
`<archive>.ll` 和 `<archive>.opt.ll`。指定 `cmd/llvmtoolexec` 时，wrapper 会在
调用 compiler 时增加内部 `-llvm-external-codegen` 协议，使 compiler 写出
`<archive>.ll` 和只含 export data 的 archive，再交给独立 `opt`/`llc`。原生
backend 对照使用一次不带 `-enablellvm` 的独立构建，不在同一 compiler invocation
中同时生成 native object 和 LLVM IR。

## 类型描述符和只读数据

进程内 backend 和外部 codegen 协议都不会跳过 compiler 的 `dumpdata` 准备阶段。`reflectdata` 仍按
原生 compiler 的方式在 `base.Ctxt.Data` 中生成最终的 `obj.LSym` 布局；随后
LLVM lowering 从带有 `TypeInfo` 的 runtime type descriptor roots 出发，收集
仅由这些 roots 的 relocation 可达的数据闭包，并将其 lower 为 LLVM constant
globals。descriptor 的主体由同一份 `rttype` runtime ABI layout 构造为 packed
named LLVM structs（例如 `%go.runtime.Type`、`%go.runtime.StructType`）；未建
schema 的辅助数据仍以 bytes/relocations 表示。这个边界刻意位于 finalized LSym，
而不是重写 reflectdata 布局逻辑。

每个被 lower 的 LSym 保留：

- descriptor 已知字段的 runtime ABI 类型、字段 offset、原始值和
  `.rodata` / `.data` / `.bss` 段属性；
- `R_ADDR`、`R_ADDROFF`、`R_METHODOFF` 及其 weak 变体的 addend 和目标；
- `DUPOK`、`LOCAL`、typelink、Go type、itab、`UsedInIface`、linkname 等 GoObj
  symbol flags；
- runtime ABIInternal 函数引用（例如 type equality closure 中的
  `runtime.memequal64`）。

LLVM IR 优先使用原生 linkage 和语义类型交接这些属性：Local 对应
`internal`，非 Local 的 DUPOK 对应 `weak`。Go type descriptor 与 itab 的
外层使用匿名 packed struct，内部字段继续使用 `%go.runtime.*` ABI 类型；descriptor
和 itab 的身份由真实的 `type:*` / `go:itab.*` global symbol 表达，避免重复生成
既长又不提供额外 LLVM 语义的 identified wrapper type。content-addressable LSym
先经过原生 `NumberSyms` 分类和 hash 计算，再用 `!goobj.content_hash` 把同一份
8/16-byte GoObj identity 交给 LLVM；writer 据此写入 Hashed64Defs/HashedDefs，
不能从 `weak`、DUPOK 或 symbol name 猜测。这样跨 package 生成的 descriptor
和 itab 仍按 Go linker 的原生规则合并。原生匿名 GoObj definition 在 LLVM IR
中使用 internal synthetic name；这类 marker payload 的链接语义由数据而非名称表达。
`!goobj.symbol.flags` 只保留 typelink、ReflectMethod、UsedInIface、linkname
以及 Local+DUPOK 重叠等没有等价 LLVM 表示的位。
普通 `R_ADDR` 由 LLVM initializer 和 target symbol 语义直接表达，不向
metadata 复制同一份信息。`R_ADDROFF` 是 GoObj 的 32 位 section offset；
`R_METHODOFF` 还额外控制 Go linker 的 dead-method elimination。LLVM 原生
relocation 均没有这两种对象格式语义，因此用 `!goobj.relocs` 精确标出它们的
relocation offset 和类型；`R_WEAKADDROFF` 的基础类型同样在该表中标为
`R_ADDROFF`。这避免让通用 LLVM backend 依赖 GoType flag、target symbol
命名、`%go.runtime.Method` 的名字或字段布局来猜测 offset relocation。
`!goobj.weak_relocs` 只记录 LLVM 无法表达的逐 relocation weak 属性。这样 LLVM
initializer 始终是地址关系的 source of truth，metadata 只承载对象格式特有的
剩余语义。零宽度的 linker
保活边 `R_KEEP` 不伪装为地址常量，而用模块级 `!goobj.keep` 关系表记录；
`gotype` aux 和 interface dead-method marker 也使用模块级关系表。表中的 source
与 target 都是对 LLVM global/function 的直接引用，不再以字符串重复符号名，
因此 LLVM 重命名和 RAUW 会同步更新这些关系；关系涉及的值同时进入
`@llvm.compiler.used`，避免 GlobalDCE 删除仅承担对象格式语义的声明。该 LLVM
特殊全局本身不生成数据，writer 再根据关系表合成 GoObj relocation/aux。最终链路仍只有
`__.PKGDEF + _go_.o` 两个有意义的 archive members；不会生成或合并 native data
object。

静态 interface conversion 还会把带有 `ItabInfo` 的 roots 纳入同一数据闭包。
itab 使用 `%go.runtime.ITab` 和按实际方法数扩展的 packed LLVM struct 表达；
固定的 `Fun[0]` 后面按 LSym 的最终大小追加 `ptr` 数组。`ptr` 与 runtime
`uintptr` 具有相同大小和对齐，因此单方法和多方法 itab 仍保持连续的 ABI
布局，同时 LLVM 能保留静态方法入口的 function-pointer 语义。方法入口仍保留
weak `R_ADDR` relocation。interface 方法调用从 itab 槽直接加载 LLVM pointer，
并以原 SSA `AuxCall` 的 ABIInternal signature 发出 indirect call。

Go linker 的 dead-method elimination 还依赖函数上的零宽度
`R_USEIFACE`、`R_USEIFACEMETHOD` 和 `R_USENAMEDMETHOD`。它们不对应 LLVM
机器指令或地址常量，因此 compiler 用 `!goobj.marker_relocs` 保存精确的
relocation type、addend 和 target name；GoObj writer 将其恢复到源函数的
relocation 列表。普通 LLVM call graph 不能替代这项 linker 契约。

non-empty interface 到另一 non-empty interface 的转换沿用 compiler 生成的
`internal/abi.TypeAssert` cache 和 `runtime.typeAssert` fallback。LLVM lowering
保留 cache 的 sequentially-consistent pointer load、pointer/uintptr probe 比较、
nil 分支和 ABIInternal runtime call；cache miss 与随后命中的 fast path 均由
runtime 测试覆盖。empty interface 到 concrete type 的 comma-ok assertion
直接比较动态 type word 并按 SSA 结果形状取回 data；empty interface 到
non-empty interface 的 comma-ok assertion 复用同一 TypeAssert cache/fallback，
并覆盖成功、类型不匹配和 nil 输入。panic-form assertion 沿用相同 fast path，
失败边分别调用 `runtime.panicdottypeE`、`runtime.panicdottypeI` 或
`runtime.panicnildottype`。

interface type switch 对 concrete cases 使用动态 type hash/type pointer 比较；
interface case 使用 compiler 生成的 `internal/abi.InterfaceSwitch` descriptor、
atomic cache probe 和 `runtime.interfaceSwitch` fallback。descriptor 作为带
`AuxGotype` 的可写 data root 进入同一 LSym-to-LLVM data closure；其尾部
interface type 指针数组按实际 case 数扩展，并由多 interface case 测试覆盖。
两个非 nil interface 的 equality 分别保留 `runtime.efaceeq` 或
`runtime.ifaceeq` ABIInternal call，以动态 type/itab 和 data words 完成比较。

这类可写 descriptor 还要求 GoObj symbol 的精确大小、对齐和 `AuxGotype`。
LLVM GoObj writer 从 global layout 写入 symbol size/alignment，排除 section
padding，并通过 `!goobj.gotype` 恢复 compiler LSym 的 Go type auxiliary。
该 auxiliary 的 target 既可以是本包定义，也可以是 external non-package
reference；后者用于 builtin 或其他 package 拥有的 type symbol。
否则 linker 无法为 `.data` 生成正确的 GC bitmap。

当前实现只 lower type-rooted data closure，尚未把所有 `dumpdata` 产物泛化为
LLVM data lowering。type descriptor 中未被当前 schema 覆盖的尾部数据，以及其他
data roots，保守地维持 bytes/relocation fallback；新增 schema 或 root 前必须先明确
其 relocation、GC 和 linker 契约，并为 writer 增加相应的 GoObj regression。

## 闭包调用 ABI

Go SSA 的 `ClosureCall` / `ClosureLECall` 不能按普通 indirect call lowering。
调用的第一个 SSA operand 是从 funcval 首字加载的 code pointer，第二个 operand
是 funcval 自身；后者不是普通 Go 参数，而是 ABIInternal 的隐藏 closure context。
LLVM lowering 在 `AuxCall` 描述的普通参数之后追加一个 `ptr nest` 参数，并在
call site 和被调函数定义上保持相同签名。GoObj target 把该参数固定放入原生
REGCTXT：amd64 使用 RDX，arm64 使用 X26，不把它计入普通参数寄存器或栈参数布局。

闭包函数定义必须同时满足 `obj.NEEDCTXT` 和 SSA `OpGetClosurePtr`；两者不一致、
closure callee/context 类型错误、code pointer 不是从同一 funcval 的单次
`uintptr` load 获得，或 closure call 使用 ABI0 时，LLVM lowering 都会
fail-fast。ABI0 的普通 static call 仍使用独立 calling convention，但当前没有
安全的 ABI0 closure-context 契约。多返回值沿用 `AuxCall` 的 ABI 结果顺序，
在 LLVM 中以 aggregate return 表达，再由 `OpSelectN` 提取；零结果和单结果不
虚构 tuple。

nil funcval 调用必须在 indirect call 前产生可恢复的 Go panic。原生 Go backend
依赖 funcval 首字的机器 load 在零地址 fault；普通 LLVM null load 则允许优化器
按 undefined behavior 处理。包含 closure code load 的 LLVM caller 因此带有
`null_pointer_is_valid` function attribute，真实 code pointer 保持单次普通
`load ptr`。load 结果直接进入 indirect call，不会成为 dead load；attribute
则阻止 LLVM 仅因 funcval 可能为 nil 就把该路径改写为 `unreachable`。最终
机器代码与原生 Go backend 一样，依赖零页访问产生 fault。

TODO：通用 `OpNilCheck` 当前仍 lower 为结果未使用的 `load volatile i8`。
`null_pointer_is_valid` 只能消除 null dereference 的 undefined behavior，不能
阻止 DCE 删除未使用的普通 load。后续应在 LLVM GoObj 路径中引入可分析的 Go
nilcheck marker/intrinsic，并实现 target-aware nilcheck analysis：在 panic
顺序、可恢复 fault、允许的 implicit-check offset 和 memory dependence 均满足时，
把显式 nilcheck 合并到后续 faulting load；否则保留确定性的 check。完成该分析
前不能删除通用 `OpNilCheck` 的 volatile side effect。

带 closure context 的 GoObj 函数发生 stack growth 时必须调用
`runtime.morestack`，使 REGCTXT 在 slow path 中保留；普通函数仍调用
`runtime.morestack_noctxt`。LLVM backend 根据 `nest` 参数选择这两个入口。
这一规则与调用点的 hidden context lowering 是同一 ABI 契约，不能只修改其中
一侧。

target prologue emission 将 morestack 重试边建模为机器 CFG loop。无 profile
时，compiler 内的 `llvmCodeGenOptions` 固定设置 `-force-loop-cold-block`，让
`MachineBlockPlacement` 按 target 已设置的极低 slow-path 边概率把 morestack
block 移到正常函数体和 `RET` 之后。保留的 `llvmtoolexec` 外部 A/B 路径传递同一
参数。这样热路径从 stack check 直接 fallthrough 到函数体，与原生 Go 的扩栈
序言布局一致；不能删除该参数后仅依赖初始 MBB 顺序。

## 固定栈槽和动态 alloca

每个仍有 use 的 SSA `OpLocalAddr` 在 LLVM entry block 预先建立唯一的固定
`alloca`。栈槽按源变量身份和位置复用，并使用 Go 类型要求的对齐；循环或分支
中的 `OpLocalAddr` 只引用该 entry-block 栈槽，不在控制流内部重复分配。没有
use 的 local 不生成 LLVM 栈槽，避免无意义地增大 frame。

GoObj stack growth、SP 恢复和当前 stack-map 路径只支持编译期固定 frame。
因此 amd64 和 arm64 GoObj target 对 variable-sized `alloca` 都确定性报错，
不能把动态栈分配交给 LLVM 通用 lowering。当前 GoObj writer 已为所有 GoObj Go
函数生成类型推导的入口 ArgsPointerMaps，并为已支持的普通 statepoint 和固定
`alloca` 生成 LocalsPointerMaps；这仍不代表已经支持完整 precise-GC stack maps、
goroutine、defer 或 panic unwind。

对应回归包括：

- `test/codegen/closure.go`：hidden context、tuple return、重复 funcval 调用和
  entry-block alloca；
- `test/llvm_closure.go`：逃逸 funcval、capture、重复调用和递归 stack growth；
- LLVM `goobj-stack-growth-metadata.ll`：`morestack` 与
  `morestack_noctxt` 的选择及 REGCTXT 保留；
- LLVM `goobj-dynamic-alloca.ll`：两个已支持 GoObj target 上的动态 alloca
  fail-fast。

本阶段验证基线为 41/41 LLVM codegen whitelist 通过，run whitelist
18/20 通过。剩余 `llvm_interface_assertion.go` 和
`llvm_interface_conversion.go` 失败来自既有 panic/traceback unwind 限制，
不是 closure ABI、alloca 或 GoObj relocation mismatch。

## LLVM IR 契约

LLVM IR 本身是 frontend 到 `llc` 的唯一配置载体。`compile` 设置 GoObj
target triple，并添加一个名为 `!goobj.config` 的 named metadata：

```llvm
target triple = "aarch64-apple-darwin-goobj"
!goobj.config = !{!0}
!0 = !{!"goallc.goobj", !"darwin", !"arm64", !"go1.27rc2",
       !"GOARM64", !"v8.0", !"build-id", !"main", !"1", !"0", !1}
!1 = !{!"regabiwrappers", !"regabiargs"}
```

`!0` 的字段顺序固定，每一项均为独立 metadata operand：

| Index | 字段 |
| --- | --- |
| 0 | schema 标识，固定为 `goallc.goobj` |
| 1–2 | `GOOS`、`GOARCH` |
| 3 | Go compiler 版本 |
| 4–5 | 架构设置的 key/value，例如 `GOARM64` / `v8.0` |
| 6 | build ID |
| 7 | package path |
| 8–9 | `main`、`shared`，值为 `0` 或 `1` |
| 10 | experiment metadata node；其中每个 operand 是一个 experiment 名称 |

experiments 不使用逗号分隔字符串，其他字段也不拼接成 Go textual object
header。进程内 binding 和保留的 `llc` driver 都会在建立 code-generation
pipeline 前读取并严格校验这些字段，然后把它们传给 GoObj writer。没有
`!goobj.config` 的普通 GoObj IR 仍可使用
既有 `-goobj-*` command-line 选项作为兼容回退。

对应 LLVM 测试为
`llvm/test/CodeGen/AArch64/goobj-ir-config.ll`。LLVM 侧实现位于
`llvm/tools/llc/llc.cpp`、`llvm/lib/CodeGen/CodeGenTargetMachineImpl.cpp`
和 `llvm/lib/MC/GoObjObjectWriter.cpp`；进程内解码入口位于 vendored
`go-llvm/SupportBindings.cpp`。

## 保留的外部 toolexec 调试链路

`cmd/llvmtoolexec` 只处理实际参数中启用了 `-enablellvm` 的 `compile`，其余
compile 和其他 Go tool 调用原样透传。它：

1. 调用 compiler 时增加内部 `-llvm-external-codegen`，由 compiler 写出仅包含
   export data 的 archive 和对应 `.ll` sidecar；
2. 调用
   `llc -load-pass-plugin=<GoALLCStatepoints> -filetype=obj`；GoObj 固定以
   CALL 起点记录 statepoint。插件的 pre-codegen callback 与 compiler
   进程内 LLVM backend 共用同一个 pass pipeline 入口，wrapper 不在命令行中
   重建 pipeline；
3. 使用 `cmd/internal/archive` 打开 compiler archive，并将对象以 `_go_.o`
   追加进去；
4. 默认删除 IR 和临时 object，`-keep-ir` 可保留 IR 供检查。

wrapper 不解析 `__.PKGDEF`、不反向识别 Go textual header，也不自行写 ar
header。使用 Go 自己的 archive writer 可保证 `__.PKGDEF` 保持第一个 member，
避免 BSD `ar` 插入 `__.SYMDEF` 后破坏 `cmd/link` 的读取约定。

package 的选择完全由 cmd/go 已有的 `-gcflags` 规则完成。无 pattern 的
`-gcflags='-enablellvm'` 只作用于命令行 package；需要选择某个依赖时使用
`-gcflags='example.com/project/internal/foo=-enablellvm'`。wrapper 不维护另一套
package 匹配规则，也不根据 `TOOLEXEC_IMPORTPATH` 猜测选择结果。实际 compile
缺少 `-enablellvm` 时完全透传。由于 compiler 的 `-V=full` probe 不携带
package gcflags，wrapper 的 compiler identity 始终纳入 LLVM backend；这样
LLVM payload 或 plugin 变化时不会复用旧的 LLVM archive，代价是同一 wrapper
下的原生 compile 也会保守地失效缓存。

`llc` 的选择顺序是 `-llc`/`GOALLC_LLC`、`GOALLC_LLVM_DIR/bin/llc`、构建
toolchain 时记录的 LLVM payload、`$GOROOT/llvm/bin/llc`。正常安装后只需传
`-toolexec`；开发者仍可用前两项覆盖 payload。

pass plugin 默认从 `llc` 所属 LLVM payload 的
`lib/GoALLCStatepoints.{dylib,so}` 查找。也可使用 `-pass-plugin` 或
`GOALLC_PASS_PLUGIN` 指定精确路径。wrapper 对选中的 compile 强制加载插件；
插件缺失时在运行 compiler 前 fail fast，不能静默绕过 pre-codegen pipeline。
插件必须来自与 `llc` 相同的 LLVM checkout/payload，不能只按 LLVM major
版本混用。`make.bash` 对 Go-owned plugin sources、实际 `llc`、`libLLVM`、
`llvm-config` 和 LLVM CMake 配置做内容哈希；输入未变时跳过 CMake，输入变化时
重建并用新 inode 原子刷新 payload 中的插件。时间戳本身不作为缓存身份。
安装后的 compile probe 把 wrapper、实际 `llc`、动态 `libLLVM` 和 plugin 的
内容纳入 Go action-cache tool ID；因此同一路径下替换 backend/plugin 会使该
wrapper 覆盖的 compile action 重新编译。

插件的功能性源码和测试位于 Go 仓库
`src/cmd/llvmplugin`，不放入 LLVM 源码树。LLVM 提供通用的
`-load-pass-plugin`、pre-codegen callback，以及 GoObj 对 Machine StackMaps
的对象格式适配；GoALLC statepoint rewrite 及其 pass 顺序都应继续在这个
Go-owned 工程中实现。当前
`runPreCodeGenPipeline` 调用 Go-owned statepoint pass：它对 Go ABI 函数执行
CFG 逆向数据流活跃性分析，为普通调用分配稳定 callsite ID，生成
`gc.statepoint` / `gc.relocate`，并识别 `gc-leaf-function`。受管指针分类当前
对 LLVM `ptr` 保守，不依赖 addrspace；活跃的 `alloca` 地址及其派生指针也
进入 `gc-live`，不在 IR 层提前猜测其最终机器位置。第一阶段对 live pointer
aggregate、`invoke`、`musttail` 和非 leaf inline asm fail closed。默认
compile 已在进程内加载 plugin adapter 并调用同一 pre-codegen callback；core
与 adapter 的分离继续支持独立单测和以后进一步静态集成。

机器位置不通过修改 LLVM 通用 `StackMaps.cpp` 截获。SSA→LLVM IR 前端为 Go ABI
函数声明 `gc "goallc"`；GoObj Go 函数默认采用原生 Go 的可扩栈策略，只有
`go-nosplit`、`go-systemstack` 这样的例外策略需要额外属性。插件负责注册对应的
GC strategy、执行 statepoint rewrite 并消费这些前端标记。在 LLVM GoObj
AsmPrinter 模块收尾阶段
读取标准 `FnInfos/CSInfos`，跳过 statepoint 的 CC、flags 和 deopt 前缀后，把
原始 GC locations 写入 MCContext。GoObj writer 在最终 layout 后完成 SP
校验、`Direct`/`Indirect` 解释、LocalsPointerMaps 和 PCDATA_StackMapIndex
编码。GoALLC 要求 StackMaps 记录 CALL 起点；map 从 CALL 开始。LLVM target
formal lowering 为每个 GoObj Go 函数推导入口参数 home，并用一个独立、零字节的
`EntryArgsStackMapID` 表达函数级 ArgsPointerMaps；该记录不是调用点，不生成
PCDATA。非 nosplit 函数在 PEI 阶段额外把 `runtime.morestack` 调用生成为物理、
root-free 的 `StackGrowthStatepointID`，从 CALL 起点选择入口 ArgsPointerMaps 和
空 locals bitmap。nosplit 函数不得包含这个 statepoint。这样普通调用与栈增长
调用仍走同一 Machine StackMaps 链路，且不依赖 return PC 反推调用范围，同时
函数级入口图不再伪装成 morestack 调用。GoObj 先写索引 0 的
`PCDATA_UnsafePoint`（当前恒为 safe 的 `-1`），再写索引 1 的
`PCDATA_StackMapIndex`；不能只写后一张表，否则 linker 会把它误认成索引 0。
`Direct SP+offset` 是栈地址本身，不表示该 slot 存有 pointer，因此不会设置
bitmap；`Indirect [SP+offset]` 才表示该槽保存指针值。最终 frame 内的间接槽
设置 LocalsPointerMaps，最终 frame 之上的 caller-owned 参数 home 或栈结果槽
设置 ArgsPointerMaps。这允许 IR 层保守追踪所有 alloca，同时避免把 alloca
对象内容误当成指针。

普通栈入参不需要额外的旁路格式。SelectionDAG formal lowering 只在 Go ABI
pointer part 是从 immutable fixed object 发出的 direct、non-extending load，
且 IR aggregate offset、ABI `PartOffset`、load size 与 object size 完全一致时，
记录这个精确 value home。若后续 `gc-live` 是该参数或其精确
`extractvalue` pointer leaf，statepoint lowering 直接把同一个 fixed frame
index 作为 indirect memory operand，允许 GC 更新该槽，并让 `gc.relocate`
从同一位置重新加载；调用前不再复制到 locals spill。merge、派生值或任一尺寸/
offset 无法证明一致时，继续走 LLVM 原有的 local statepoint spill。这仍是标准
SelectionDAG statepoint operand，不引入 `byval`/`sret`，也不后移 GoALLC 生成
LLVM IR 的时机。当前 GoObj/运行时精确资格边界仍是 darwin/arm64；相同 fixed
home 选择路径另有 AArch64 与 X86 MIR lit 覆盖。

同源可执行回归 `test/abi/llvm_args_results.go` 还覆盖四种组合：第一个栈槽中的
单个 pointer、由一个 non-pointer word 分隔的两个 pointer、含两个 pointer
leaf 的三字 aggregate，以及该 aggregate 栈入参与两个 caller-owned pointer
结果槽同时溢出。每个 callee 在读取这些 pointer 前执行 `runtime.GC`，caller
随后再次执行 GC 并核对 pointer 身份、指向值和 scalar payload。差异测试同时
固定 native/GoALLC 的 ArgsPointerMaps、LocalsPointerMaps、PCDATA 查询结果和
最终 MIR，因此运行成功不能替代元数据证据。

AArch64 GoObj 的 prologue 采用 Go arm64 栈链约定，而不是平台 ABI 的
in-frame `(FP, LR)` record。若最终物理 frame 大小为 `StackSize`，则
`LR` 位于 `SP+0`，当前函数为未来 callee 写入的 FP link 位于 `SP-8`，
caller 已写入的 FP link 占据 frame 顶部 `SP+StackSize-8`。因此 GoObj
writer 直接由最终 `StackSize` 推导 `_func.locals=StackSize-8`，locals
pointer bitmap 只描述 `[SP+8, SP+StackSize-8)`。这里不从 IR lowering
额外传递 locals size：底部 LR 和顶部 FP link 是 target frame layout
已经确定的两个保留槽。与原生 arm64 backend 一样，小 frame 用 pre-index
`STR LR` 原子地保存 LR 并移动 SP；超过 `0xf0` 的大 frame 则先计算 NewSP，
在移动 SP 前保存 `(FP, LR)`，避免异步 traceback 看到半构造的 frame。

当前 ArgsPointerMaps 的第 0 项描述 ABIInternal/ABI0 的入口 pointer 参数
home；普通 statepoint 则按最终机器位置把 caller-owned 参数/结果区中的间接
pointer 槽写入对应 ArgsPointerMaps 项，其中可证明的栈入参直接使用原 fixed
home，而不是另建 locals spill。Args 和 Locals 表按完整 pair 一起去重，
并由同一个 `PCDATA_StackMapIndex` 选择。writer 不按声明类型预先标记所有结果槽：
只有 statepoint 真实报告其中存有 live pointer 的栈结果槽才置位，尚未物化或尚未
初始化的结果槽保持为空。StackObjects 仍未生成；追踪 alloca 地址也不能描述
alloca 对象内部保存的指针字段。

`make.bash` 会使用同一个 LLVM payload 的 CMake config 构建插件，并以新 inode
原子安装。规范的 LLVM payload、plugin、Go 构建和验证流程见
[goallc-build.md](goallc-build.md)：

```sh
cd "$GOROOT/src"
./make.bash \
  -llvm-dir=/path/to/goallc-llvm \
  -llvm-version=23 \
  -llvm-link=dynamic
```

安装结果为 Darwin 上的
`$LLVM_PAYLOAD/lib/GoALLCStatepoints.dylib` 或 Linux 上的
`$LLVM_PAYLOAD/lib/GoALLCStatepoints.so`。不要用其他 LLVM 安装构建后再复制
产物；pass plugin 的 C++ ABI 必须和 compiler 所链接及 `llc` 所使用的 LLVM
payload 精确匹配。

## 最小使用方式

假定 `$GOROOT` 是本仓库构建出的 Go toolchain，并且 `make.bash -llvm-dir=...`
已经记录对应 LLVM payload：

```sh
cd /path/to/simple-main-package
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm' \
  -o app .
./app
```

进程内参数与保留的 wrapper 参数边界如下：

| 旧 `llvmtoolexec` 参数 | 进程内 compiler 参数 | 行为 |
| --- | --- | --- |
| `-opt-passes` | `-llvm-opt-passes` | 默认 `default<O2>`；`none` 跳过 IR 优化 |
| `-keep-ir` | `-llvm-keep-ir` | 保留 `<archive>.ll` 和 `<archive>.opt.ll` |
| 固定 llc 参数 | 无需指定 | 同样启用 `-trap-unreachable`、`-disable-machine-cse` 和默认的 `-disable-lsr` |
| `-llc` / `-opt` / `-pass-plugin` | 仅外部模式 | 进程内固定使用 toolchain payload 或静态链接实现 |
| `-enable-lsr` | 仅外部模式 | 进程内固定禁用尚不安全的 LSR |
| `-native-package` | `-gcflags` package pattern | 只给需要 LLVM 的 package 传 `-enablellvm` |

例如保留进程内优化前后的 IR：

```sh
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm -llvm-keep-ir' \
  .
```

例如只让一个依赖走 LLVM，可使用：

```sh
"$GOROOT/bin/go" build \
  -gcflags='example.com/project/internal/foo=-enablellvm' \
  .
```

反过来，若大部分 package 走 LLVM、少数 package 保留原生 backend，可利用
`-gcflags` 的“最后一个匹配规则生效”语义表达旧 `-native-package`：

```sh
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm' \
  -gcflags='runtime=' \
  .
```

需要与旧链路对比时只需增加 `-toolexec`，package 仍使用同一个
`-enablellvm` 选择：

```sh
TOOLDIR=$("$GOROOT/bin/go" env GOTOOLDIR)
"$GOROOT/bin/go" build \
  -toolexec="$TOOLDIR/llvmtoolexec -opt-passes=default<O2> -keep-ir" \
  -gcflags='all=-enablellvm' \
  .
```

开发时至少运行：

```sh
cd "$GOROOT/src"
go test cmd/internal/llvmbackend cmd/llvmtoolexec cmd/go/internal/work cmd/dist

cd /path/to/llvm-project
llvm/cmake-build-debug/bin/llvm-lit -sv \
  llvm/test/CodeGen/AArch64/goobj-ir-config.ll
```

此外应以一个简单 `main` package 分别执行进程内 `go build` 和保留的
`go build -toolexec`，并运行两个产物。前者验证内存 IR、进程内 plugin/codegen、
compiler archive writer 和 linker；后者保留外部工具链的 A/B 证据。

## 标准库的分层构建和测试

扩大到标准库时必须记录 LLVM 实际覆盖的依赖范围。以下三种命令不能混称为
“标准库使用 LLVM”。每次使用新的 `GOCACHE`，避免上一层生成的 archive 掩盖
本层的编译边界；`-x -work` 和 compiler 的 `-llvm-keep-ir` 可在调查失败时追加。
本节各层默认只传 `-enablellvm`。需要和外部 pipeline 做对比时，使用前文保留的
toolexec 命令，并保持相同的 package pattern。

公共设置如下。`LLVM_ROOT` 必须是构建 toolchain 时使用的项目 payload；正式
Linux 资格还必须核对 release manifest 的 revision 和 `llvm_dirty=false`。
Darwin 本地 dirty payload 只能记为开发验证。

```sh
GOROOT=/path/to/goallc-go-worktree
LLVM_ROOT=/path/to/goallc-llvm-payload
PKG=unicode/utf16
```

### 只编译入口标准库包

```sh
CACHE=$(mktemp -d)
env GOROOT="$GOROOT" GOCACHE="$CACHE" GOALLC_LLVM_DIR="$LLVM_ROOT" \
  "$GOROOT/bin/go" test -count=1 -timeout=2m \
  -gcflags="$PKG=-enablellvm" \
  "$PKG"
```

这里的“入口”是 cmd/go 的 package pattern：目标包及其 test variant 使用 LLVM，
普通依赖、`runtime` 和 `testing` 仍使用原生 backend。它适合先验证单个包的
frontend、GoObj、链接和运行语义，但不能证明其标准库依赖闭包使用了 LLVM。

### 编译标准库依赖闭包，但保留 runtime 闭包

下面从目标包的普通标准库依赖中减去 `runtime` 的依赖闭包。`testing` 和仅由测试
引入的依赖不在普通 `go list -deps "$PKG"` 集合中，也保持原生 backend。

```sh
CACHE=$(mktemp -d)
RUNTIME_DEPS=$(mktemp)
env GOROOT="$GOROOT" GOCACHE="$CACHE" \
  "$GOROOT/bin/go" list -deps \
  -f '{{if .Standard}}{{.ImportPath}}{{end}}' runtime |
  LC_ALL=C sort -u >"$RUNTIME_DEPS"

LLVM_GCFLAGS=()
while IFS= read -r dep; do
  test -n "$dep" || continue
  if ! grep -Fxq "$dep" "$RUNTIME_DEPS"; then
    LLVM_GCFLAGS+=("-gcflags=$dep=-enablellvm")
  fi
done < <(
  env GOROOT="$GOROOT" GOCACHE="$CACHE" \
    "$GOROOT/bin/go" list -deps \
    -f '{{if .Standard}}{{.ImportPath}}{{end}}' "$PKG" |
    LC_ALL=C sort -u
)

env GOROOT="$GOROOT" GOCACHE="$CACHE" GOALLC_LLVM_DIR="$LLVM_ROOT" \
  "$GOROOT/bin/go" test -count=1 -timeout=2m \
  "${LLVM_GCFLAGS[@]}" \
  "$PKG"
```

这个层次用来暴露入口包没有触及的公共依赖能力，例如 atomics、sync 和 io；遇到
未支持 SSA op 时应归为 frontend 能力缺口，不要通过包名排除来制造假通过。

### 编译 runtime/testing 在内的完整测试闭包

```sh
CACHE=$(mktemp -d)
env GOROOT="$GOROOT" GOCACHE="$CACHE" GOALLC_LLVM_DIR="$LLVM_ROOT" \
  "$GOROOT/bin/go" test -count=1 -timeout=2m \
  -gcflags='all=-enablellvm' \
  "$PKG"
```

这一层包含 `runtime`、`testing`、测试入口和全部依赖；只有它通过才能声称完整测试
闭包使用 LLVM。它不是入口包 smoke test 的同义词。

### 包级 CI 策略

`test/llvm_stdlib_packages.json` 单独管理入口包层次的标准库白名单和黑名单。
精确白名单优先于 `*` 黑名单；白名单包是 CI 的 required tests，黑名单包不运行，
并保留最早失败边界或“尚未资格化”的原因。CI 整个任务共享一个隔离
`GOCACHE`；目标包、`-gcflags` 和 compiler backend identity 都进入 cmd/go action
ID，因此包和构建模式仍使用不同缓存条目。工具链、payload 或同一路径下的 pass
plugin 发生变化时会使 LLVM action 失效。任务使用 `default<O2>` 和
`-gcflags='all=-enablellvm'`，因此目标包、测试入口、`runtime`、`testing`
及完整依赖闭包都由 LLVM backend 编译。每个白名单包会在独立的
`go test -count=1` 进程中运行并共享编译缓存。runner 先预热 LLVM runtime，再按
`NumCPU()/2` 并行运行 package；每个 cmd/go 使用 `-p=1` 仅限制 package 构建并发，
不会改变测试二进制的 `GOMAXPROCS` 或 `-test.parallel`。任一白名单包失败都会使
门禁失败。

本地使用与 CI 相同的策略运行器：

```sh
env GOALLC_RUN_LLVM_STDLIB=1 GOALLC_LLVM_DIR="$LLVM_ROOT" \
  "$GOROOT/bin/go" test cmd/internal/testdir \
  -run '^TestLLVMStdlib$' -count=1 -timeout=100m -v
```

### Darwin/arm64 开发载荷阶段结果

2026-08-11 的本地扩面使用基于正式 v8 加 LLVM #64 的 Darwin/arm64 开发载荷，
不能替代 Linux 正式 v8 资格。入口包共享同一个隔离 `GOCACHE`，
`default<O2>` 和上述入口包命令，并在编译成功后运行完整包用例。

以下 62 个包完成了 LLVM 编译、GoObj/archive、链接和包用例运行：

```text
cmp container/heap container/list container/ring
crypto/md5 crypto/rand crypto/sha1 crypto/sha256 crypto/sha512 crypto/subtle
encoding/ascii85 encoding/base64 encoding/binary encoding/csv encoding/hex
encoding/base32 encoding/gob encoding/json encoding/pem encoding/xml
go/scanner go/token hash hash/adler32 hash/crc32 hash/crc64 hash/maphash
hash/fnv html image image/color mime mime/quotedprintable
math math/bits math/cmplx math/rand math/rand/v2 net/netip net/url path
regexp/syntax sort strconv unicode unicode/utf8 unicode/utf16
archive/tar archive/zip
bufio bytes compress/gzip compress/zlib index/suffixarray io io/fs
path/filepath regexp strings text/scanner text/tabwriter text/template/parse
```

本轮通用修复后重新使用空缓存验证了后新增的 15 个包。其中：

- `encoding/base32`、`math/rand/v2`、`net/netip`、`image`、`sort`、`hash/fnv`、
  `encoding/json`、`encoding/xml`、`encoding/pem`、`net/url`、`math/rand`、
  `archive/tar` 和 `archive/zip` 在保留 Go 栈地址观察的 `OpConvert` lowering 后，
  均以 `default<O2>` 完整通过；这说明原先的数据破坏和超时来自同一个
  `opt/statepoint` 地址活跃性问题，而不是各包的运行语义；
- `regexp/syntax` 和 `encoding/gob` 越过了 O2 推断的 `readnone` 参数属性；
  statepoint verifier 和 SelectionDAG 均原生支持这一非 ABI 属性，因此插件与
  `readonly` 等属性一样直接保留它；
- `encoding/gob` 还暴露了一个独立的聚合标量化问题：对部分初始化的
  `insertvalue poison/undef` 聚合，插件曾为未定义指针叶制造 `extractvalue` 并
  错误加入 `gc-live`。SelectionDAG 合法删除 undef spill 后，GoObj 栈图仍会扫描
  未初始化槽。现在只用 `FindInsertedValue` 识别 poison/undef 叶；已定义叶仍保留
  各自的 `extractvalue` SSA 身份，避免 statepoint relocation 修复把同一源值的
  其他使用一并改写。常量 poison/undef 不进入活根；`TestTypeRace` 50 次和完整
  `encoding/gob` 5 次复测均通过。

### Linux/amd64 正式 v8 资格复测

2026-08-11 在 Ubuntu 22.04 x86-64 上使用 PR 121 的 `de9dde3b60` 和正式 release
`goallc-llvm23.1.0-20260811T022435Z`（revision
`90e5e5c7c626e3072fc77ce69cb42d8c7bb1b4a4`）复测了全部 28 个有精确降级记录的
入口包。每包先独立运行三次；通过的 20 个候选又追加两次，因此以下包均为 5/5：

```text
archive/tar archive/zip compress/gzip
encoding/base32 encoding/binary encoding/gob encoding/json encoding/pem encoding/xml
go/token hash/fnv hash/maphash image mime/quotedprintable
net/netip net/url regexp regexp/syntax sort strings
```

这些包不再需要公共或 linux/amd64 降级。公共白名单由 46 个增加到 60 个；扣除
当前 5 个 linux/amd64 平台降级后，该平台 CI 实际要求 55 个包连续三次通过。
剩余失败均保留在最早边界：

- `bytes`：`llc/GoObj` 拒绝含无效参数指针槽的栈增长 statepoint；
- `compress/flate`：X86 与 AArch64 都在 `compress/flate.testBlock` 的
  SelectionDAG instruction selection 中断言；
- `crypto/md5`：链接后的测试在 `runtime.convTnoptr` 附近发生 interface 数据破坏；
- `crypto/sha1`：`blockAVX2` 栈增长时报告未类型化参数 frame 缺少 stack map；
- `io`：三次中两次通过，一次 `TestPipeConcurrent/Write` 把已写数据替换为零字节，
  属于仍可复现的间歇性 runtime semantics 问题，不能升白；
- `math`：x86-64 lowering 缺少 `ftrunc` 和 `ffloor` libcall；
- `math/rand`、`math/rand/v2`：x86-64 lowering 缺少 `ffloor` libcall，Linux arm64
  仍需由 CI 之外的显式资格测试确认，所以暂不升入公共白名单。

当前入口包扩面的剩余失败按最早边界记录如下：

- `compress/flate`：`llc` 在 `compress/flate.testBlock` 的 X86/AArch64
  SelectionDAG instruction selection 中断言失败，属于 `llc/GoObj` 的 llc 子类；
- 完整闭包已越过 atomic 8/64、`Mul{32,64}uover`、`GetClosurePtr`、`GetG`、
  `PubBarrier`、prefetch，以及仅在 LLVM IR descriptor 重构时对预声明别名
  `TypeInfo` 视图的 canonicalization；原生 `reflectdata` 逻辑保持不变。当前停止在
  自动生成调用与已定义 `runtime.growslice` 的 LLVM 函数类型冲突。该问题属于
  runtime ABI/声明的系统性建模边界，不在入口包扩面中旁路。

amd64 的 `GetG` 目前只接受 ABIInternal 并读取 R14。ABI0 在原生 backend 中从
TLS 读取 g，并在 ABIInternal/ABI0 跨越处显式修复 R14；LLVM 尚未建模这套转换，
因此 amd64 ABI0 遇到 `OpGetG` 会 fail-closed，而不是误读一个未建立契约的 R14。

调查 O2/statepoint 失败时，可保留完全相同的入口包范围，只移除优化 pipeline
形成对照；这只是分类命令，不能作为 O2 资格结果：

```sh
CACHE=$(mktemp -d)
env GOROOT="$GOROOT" GOCACHE="$CACHE" GOALLC_LLVM_DIR="$LLVM_ROOT" \
  "$GOROOT/bin/go" test -count=1 -timeout=2m \
  -gcflags="$PKG=-enablellvm -llvm-opt-passes=none" \
  "$PKG"
```

### 失败分类和证据

按最早出现错误的语义边界记录失败，并同时保留后续症状：

- `frontend`：compile 在产生可用 IR 前拒绝 SSA op、ABI 或 closure 模型；
- `opt/statepoint`：IR verifier、O2 pipeline、statepoint plugin 或 GC liveness 失败；
- `codegen/GoObj`：机器 lowering、GoObj symbol/relocation/aux data 与 frontend 契约不符；
- `archive/link`：archive member、重复定义、relocation distance 或 Go linker 失败；
- `runtime semantics`：成功链接后出现错误结果、panic、崩溃、死锁或 race；
- `environment/timeout`：payload、loader、cache、宿主资源或独立超时问题。

超时前已有错误栈或语义死锁时，根因仍记为 `runtime semantics`，timeout 只是症状。
同理，若 consumer relocation 记录的 `(PkgIdx, SymIdx)` 名称正确，但 provider
archive 同一 class index 指向另一个定义，根因是 `llc/GoObj`，不能归为运行时
随机错误。报告至少包含 Go commit、payload manifest、OS/arch、scope、package、
优化 pipeline、原生命令对照，以及 IR、`go tool objview -json`、link/runtime 中
实际到达的最深证据。

## 回归测试机制

GoALLC 不维护一套与 Go 仓库重复的测试源码。LLVM 测试由
`cmd/internal/testdir` 发现并复用 `$GOROOT/test` 中已有的测试：

- codegen 候选是 `test/codegen` 下 recipe 为 `// asmcheck` 的文件；
- run 候选是 testdir 原本扫描目录中 recipe 为 `// run` 的文件；
- `test/llvm_tests.json` 分别维护 codegen 和 run 的白名单、灰名单与黑名单；
- codegen source 出现 `// LLVM-OPT` directive 时，runner 会读取进程内
  `default<O2>` pipeline 留下的 `.opt.ll`，并用 `LLVM-OPT` prefix 检查；
- 白名单是当前必须通过的用例，任何失败都会使 CI 失败；
- 灰名单是可以执行但尚未要求通过的用例，runner 会记录每个失败和汇总，
  并为每个用例明确输出 `PASS`、`FAIL (allowed)` 或 `SKIP`，但不会因此使
  CI 失败；验证稳定后通过把精确条目加入白名单来提升覆盖；
- 黑名单完全不执行，用于已知会超时、耗尽内存、超过一分钟 CI 单例预算，
  或当前统一关闭的 defer/recover 用例；runner 会校验 blacklist reason
  明确说明 timeout、OOM、slow 或 defer/recover，并在日志中输出 `NOT RUN`；
- 三类的匹配优先级为黑名单、精确白名单、灰名单。灰名单可以用 glob 覆盖
  尚未支持的范围，黑名单中的精确资源限制或 defer/recover 条目仍能阻止执行；
- `platform_graylist` 把公共白名单项在指定平台降为灰名单，普通编译或运行失败
  不再使用 platform blacklist；如确需使用，`platform_blacklist` 遵循相同的
  资源限制和暂时关闭特性规则；
- runner 会拒绝拼错的白名单、无匹配项的灰/黑名单，以及未被三类之一分类的
  候选，并报告白、灰、黑三类文件数。
- Linux CI 将策略统计、必过项失败、灰名单逐用例结果和黑名单未运行原因写入
  GitHub Actions Job Summary；完整命令输出仍保留在 step 日志中。

### LLVM IR codegen 检查

codegen 白名单文件直接在原 Go 源码中使用 LLVM `FileCheck` 指令，例如：

```go
// LLVM-DAG: define goabiinternal { i64, i8 } @codegen.div_ndivis6_int64
// LLVM-DAG: sdiv i64
// LLVM-DAG: srem i64
```

runner 使用 `compile -enablellvm -llvm-keep-ir` 完整生成 GoObj archive，并读取
compiler 留下的优化前 `.ll`：

```sh
FileCheck --check-prefix=LLVM test/codegen/example.go < package.a.ll
```

这条命令已经在 compiler 内完成优化、callback 和 codegen，但 codegen 测试只对
保留的 LLVM IR 做断言，不链接或运行 archive。可使用
`LLVM:`、`LLVM-LABEL:`、`LLVM-DAG:`、`LLVM-NOT:` 等标准 FileCheck
指令。原有 asmcheck parser 会忽略这些独立注释行，社区 Go 的机器码检查仍
按原方式运行。每个 codegen 白名单文件至少要包含一条 `// LLVM...`
指令。

`FileCheck` 的选择顺序为 `GOALLC_FILECHECK`、
`$GOALLC_LLVM_DIR/bin/FileCheck`、`$GOROOT/llvm/bin/FileCheck`。

### LLVM run 检查

run 白名单和灰名单都不增加新的 recipe，也不复制测试源码。runner 仍解析原文件的
`// run` 参数、build constraint、超时和期望输出。LLVM 的 `TestLLVM` 薄入口复用
原 `Test` 的 `test.run()`，在原有 `go run` 命令上增加 `-gcflags=all=-enablellvm`
和 `-ldflags=-w`：

```text
原有 // run 用例
  -> go run -gcflags=all=-enablellvm <原 recipe 文件和参数>
  -> compile 内存 IR + default<O2>
  -> 进程内 plugin callback + TargetMachine codegen
  -> compiler archive writer 写入 _go_.o
  -> cmd/link
  -> 运行并沿用原 testdir 输出检查
```

运行检查只把 toolchain 记录的 LLVM payload 交给 compiler，不显式选择 plugin、
`opt`、`llc` 或 toolexec。IR/codegen 检查仅额外从同一 payload 解析 FileCheck。

### 扩大覆盖范围

新增覆盖时：

1. 先确认整个现有测试文件都能由当前 LLVM lowering 处理；
2. codegen 文件在源码中增加针对 LLVM IR 的 FileCheck 指令；
3. 新候选默认由灰名单 glob 执行；修复后在 `test/llvm_tests.json` 中增加精确
   白名单项和简短能力说明；
4. runtime 文件从灰名单提升到白名单时，不修改其原有 `// run` recipe；
5. 运行 LLVM 定向测试，并同时运行对应的原生 asmcheck/run 测试。

定向运行完整 LLVM 策略（白名单和灰名单执行，黑名单跳过）：

```sh
go test cmd/internal/testdir -run='^TestLLVM$' -v
```

`TestLLVM` 和原生 `Test` 共用 `testdir_test.go` 的目录扫描、recipe 解析和用例调度；
LLVM 模式只从策略文件选择用例，并向原有 `go run` 命令增加 backend 参数。只运行
一个 LLVM 用例时，可继续在 `-run` 中追加原 testdir 的 slash-separated 名称。
普通失败留在灰名单；只有确认会导致超时或 OOM 时才进入黑名单。

## 当前范围与后续工作

- 当前 target triple 仅配置了 `darwin/arm64` 和 `linux/amd64`。
- LLVM SSA lowering 仍不完整；复杂 SSA op、GC pointer liveness、defer/panic、
  closure、完整 ABI/DWARF 等尚未达到通用正确性。
- 当前 interface 范围包括 compiler 能静态生成 itab 的 concrete-to-interface
  conversion、对应的 ABIInternal 间接方法调用，以及 non-empty
  interface-to-interface conversion；empty-interface 到 concrete/non-empty
  interface 的 comma-ok/panic-form assertion，以及包含 concrete/interface cases
  的 type switch 也已覆盖。
- 每个经 LLVM 替换的 package 都必须由 `llc` 生成完整的 `_go_.o`，不能与
  原生 compiler object 混合。
- `!goobj.config` 是这条开发链路的稳定交接点。新增 header 配置时应新增独立
  metadata 字段和相应 `llc` 验证，不要恢复 wrapper 中的 header 解析逻辑。
- 类型 descriptor data 已覆盖，但这不等同于通用 static-data lowering；目前仅
  支持 type-rooted readonly/data closure 中已验证的 relocation 类型。

相关合入记录：Go [#3](https://github.com/goallc/go/pull/3)，LLVM
[#3](https://github.com/goallc/llvm-project/pull/3)。
