# GoALLC：LLVM IR 到 GoObj 的开发链路

本文记录当前已合入的、用于开发 Go SSA 到 LLVM lowering 的最小端到端
链路。它让一个简单 Go package 经由 LLVM IR、`llc` 和 GoObj 进入正常的
Go package archive，最终仍由社区 Go linker 链接。

这是一条受限的开发路径，不是社区 Go 后端的替代品。完整的项目背景、构建
基线和更早的试验记录见 [../GOALLC.md](../GOALLC.md)。

## 数据流

```text
go build -toolexec=llvmtoolexec
  -> llvmtoolexec 拦截选中的 compile 调用
  -> compile -enablellvm -llvmironly
       -> __.PKGDEF + <archive>.ll
  -> llc -load-pass-plugin=<GoALLCStatepoints> -filetype=obj <archive>.ll
       -> Go 仓库维护的 pre-codegen pipeline
       -> LLVM GoObj
  -> cmd/internal/archive 将 GoObj 追加为 _go_.o
  -> cmd/link 读取 __.PKGDEF 和 _go_.o
```

`-llvmironly` 必须和 `-enablellvm` 一起使用。它使 `compile` 在写出 LLVM
IR 和 Go archive 的 `__.PKGDEF` 后结束，不再生成原生 linker object。因而
对被替换的 package 而言，archive 中唯一的 `_go_.o` 必须是 `llc` 生成的
GoObj；`__.PKGDEF` 仍完全由 Go compiler 生成并位于 archive 的第一个成员。

## 类型描述符和只读数据

`-llvmironly` 不会跳过 compiler 的 `dumpdata` 准备阶段。`reflectdata` 仍按
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
既长又不提供额外 LLVM 语义的 identified wrapper type。`!goobj.symbol.flags` 只保留 typelink、
UsedInIface、linkname 以及 Local+DUPOK 重叠等没有等价 LLVM 表示的位。
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

## 固定栈槽和动态 alloca

每个仍有 use 的 SSA `OpLocalAddr` 在 LLVM entry block 预先建立唯一的固定
`alloca`。栈槽按源变量身份和位置复用，并使用 Go 类型要求的对齐；循环或分支
中的 `OpLocalAddr` 只引用该 entry-block 栈槽，不在控制流内部重复分配。没有
use 的 local 不生成 LLVM 栈槽，避免无意义地增大 frame。

GoObj stack growth、SP 恢复和当前 stack-map 路径只支持编译期固定 frame。
因此 amd64 和 arm64 GoObj target 对 variable-sized `alloca` 都确定性报错，
不能把动态栈分配交给 LLVM 通用 lowering。当前 GoObj writer 的 args/locals
pointer maps 仍为空；本实现只保证现有固定栈槽的 dominance、frame placement
和 stack-growth 约束，不代表已经支持完整 precise-GC stack maps、goroutine、
defer 或 panic unwind。

对应回归包括：

- `test/codegen/closure.go`：hidden context、tuple return、重复 funcval 调用和
  entry-block alloca；
- `test/llvm_closure.go`：逃逸 funcval、capture、重复调用和递归 stack growth；
- LLVM `goobj-stack-growth-metadata.ll`：`morestack` 与
  `morestack_noctxt` 的选择及 REGCTXT 保留；
- LLVM `goobj-dynamic-alloca.ll`：两个已支持 GoObj target 上的动态 alloca
  fail-fast。

本阶段验证基线为 41/41 LLVM codegen whitelist 通过，runtime whitelist
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
header。`llc` 在建立 code-generation pipeline 前读取并严格校验这些字段，
然后把它们传给 GoObj writer。没有 `!goobj.config` 的普通 GoObj IR 仍可使用
既有 `-goobj-*` command-line 选项作为兼容回退。

对应 LLVM 测试为
`llvm/test/CodeGen/AArch64/goobj-ir-config.ll`。LLVM 侧实现位于
`llvm/tools/llc/llc.cpp`、`llvm/lib/CodeGen/CodeGenTargetMachineImpl.cpp`
和 `llvm/lib/MC/GoObjObjectWriter.cpp`。

## toolexec wrapper

`cmd/llvmtoolexec` 只处理实际参数中启用了 `-enablellvm` 的 `compile`，其余
compile 和其他 Go tool 调用原样透传。它：

1. 保留调用方传入的 `-enablellvm`，并增加 `-llvmironly`；
2. 调用
   `llc -load-pass-plugin=<GoALLCStatepoints> -filetype=obj`；GoObj 固定以
   CALL 起点记录 statepoint。插件的
   pre-codegen callback 是当前外部链路与未来 compiler 进程内 LLVM
   集成共用的 pass pipeline 入口，wrapper 不在命令行中重建 pipeline；
3. 使用 `cmd/internal/archive` 打开 compiler archive，并将对象以 `_go_.o`
   追加进去；
4. 默认删除 IR 和临时 object，`-keep-ir` 可保留 IR 供检查。

wrapper 不解析 `__.PKGDEF`、不反向识别 Go textual header，也不自行写 ar
header。使用 Go 自己的 archive writer 可保证 `__.PKGDEF` 保持第一个 member，
避免 BSD `ar` 插入 `__.SYMDEF` 后破坏 `cmd/link` 的读取约定。

package 的选择完全由 cmd/go 已有的 `-gcflags` 规则完成。无 pattern 的
`-gcflags=-enablellvm` 只作用于命令行 package；需要选择某个依赖时使用
`-gcflags='example.com/project/internal/foo=-enablellvm'`。wrapper 不维护另一套
package 匹配规则，也不根据 `TOOLEXEC_IMPORTPATH` 猜测选择结果。实际 compile
没有 `-enablellvm` 时完全透传，并且其 action cache identity 不依赖 LLVM
backend；只有启用 LLVM 的 compile，其 `-V=full` probe 才携带 `-enablellvm`
并纳入 backend identity。

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
安装后的 enabled compile probe 把 wrapper、实际 `llc`、动态 `libLLVM` 和
plugin 的内容纳入对应 package 的 Go action-cache tool ID；因此同一路径下替换
backend/plugin 也会使启用了 `-enablellvm` 的 package 重新编译，而未启用的
package 保持原生 compiler cache key。

插件的功能性源码和测试位于 Go 仓库
`src/cmd/llvmplugin`，不放入 LLVM 源码树。LLVM 只提供通用的
`-load-pass-plugin` 和 pre-codegen callback；GoALLC statepoint rewrite 及其
pass 顺序都应继续在这个 Go-owned 工程中实现。当前
`runPreCodeGenPipeline` 调用 Go-owned statepoint pass：它对 Go ABI 函数执行
CFG 逆向数据流活跃性分析，为普通调用分配稳定 callsite ID，生成
`gc.statepoint` / `gc.relocate`，并识别 `gc-leaf-function`。受管指针分类当前
对 LLVM `ptr` 保守，不依赖 addrspace；活跃的 `alloca` 地址及其派生指针也
进入 `gc-live`，不在 IR 层提前猜测其最终机器位置。第一阶段对 live pointer
aggregate、`invoke`、`musttail` 和非 leaf inline asm fail closed。未来
compile 进程内集成 LLVM 时直接复用该 core 入口，不经过 plugin adapter。

机器位置不通过修改 LLVM 通用 `StackMaps.cpp` 截获。SSA→LLVM IR 前端为 Go ABI
函数声明 `gc "goallc"` 和 `go-stack-growth-statepoint`，插件只负责注册对应的
GC strategy 与 `GCMetadataPrinter::emitStackMaps`，并消费这些前端标记。在
AsmPrinter 模块收尾阶段
读取标准 `FnInfos/CSInfos`，跳过 statepoint 的 CC、flags 和 deopt 前缀后，把
原始 GC locations 写入 MCContext。GoObj writer 在最终 layout 后完成 SP
校验、`Direct`/`Indirect` 解释、LocalsPointerMaps 和 PCDATA_StackMapIndex
编码。GoALLC 要求 StackMaps 记录 CALL 起点；map 从 CALL 开始。前端添加的
`go-stack-growth-statepoint` 属性使 LLVM 在 PEI 阶段把
`runtime.morestack` 调用生成为物理 MIR `STATEPOINT`。其 deopt 和 GC alloca
区为空，GC pointer 区则记录类型推导出的入口参数 home。它从 morestack CALL
起点选择入口 ArgsPointerMaps 和空 locals bitmap，因此普通调用与栈增长调用
走同一 Machine StackMaps 链路，且不依赖 return PC 反推调用范围。已有
Machine `STATEPOINT` 但缺少该前端属性时，
LLVM target lowering 会 fail closed；不再使用 slow-path reset label 兼容普通
morestack CALL。GoObj 先写索引 0 的
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

在构建 Go toolchain 前，使用同一个 LLVM payload 的 CMake config 构建、
测试并安装插件：

```sh
LLVM_PAYLOAD=/path/to/goallc-llvm
PLUGIN_BUILD=/path/to/empty/plugin-build

cmake -S "$GOROOT/src/cmd/llvmplugin" -B "$PLUGIN_BUILD" -G Ninja \
  -DLLVM_DIR="$LLVM_PAYLOAD/lib/cmake/llvm" \
  -DCMAKE_INSTALL_PREFIX="$LLVM_PAYLOAD"
cmake --build "$PLUGIN_BUILD"
ctest --test-dir "$PLUGIN_BUILD" --output-on-failure
cmake --install "$PLUGIN_BUILD"
```

安装结果为 Darwin 上的
`$LLVM_PAYLOAD/lib/GoALLCStatepoints.dylib` 或 Linux 上的
`$LLVM_PAYLOAD/lib/GoALLCStatepoints.so`。不要用其他 LLVM 安装构建后再复制
产物；pass plugin 的 C++ ABI 必须和负责加载它的 `llc` 精确匹配。

## 最小使用方式

假定 `$GOROOT` 是本仓库构建出的 Go toolchain，并且 `make.bash -llvm-dir=...`
已经记录对应 LLVM payload：

```sh
cd /path/to/simple-main-package
TOOLDIR=$("$GOROOT/bin/go" env GOTOOLDIR)
"$GOROOT/bin/go" build \
  -toolexec="$TOOLDIR/llvmtoolexec" \
  -gcflags=-enablellvm \
  -o app .
./app
```

也可以用 `"$GOROOT/bin/go" tool -n llvmtoolexec` 查看安装路径。需要覆盖 payload
或保留 IR 时，把参数放进同一个 `-toolexec` 值：

```sh
"$GOROOT/bin/go" build \
  -toolexec="'$GOROOT/pkg/tool/darwin_arm64/llvmtoolexec' '-llc=/path with spaces/llc' -keep-ir" \
  -gcflags=-enablellvm \
  .
```

这里使用 cmd/go 的 quoted command syntax，因此 wrapper flags 和带空格路径不会
被 shell 或 cmd/go 二次拆错。`-keep-ir` 留下的 IR 路径为 compile 输出 archive
路径加 `.ll` 后缀。启用了 `-enablellvm` 的真实 compile 缺少 `llc`/plugin 时
仍然 fail closed；非 compile、没有 `-enablellvm` 的 package 和普通 probe
透明透传。

例如只让一个依赖走 LLVM，可使用：

```sh
"$GOROOT/bin/go" build \
  -toolexec="$TOOLDIR/llvmtoolexec" \
  -gcflags='example.com/project/internal/foo=-enablellvm' \
  .
```

开发时至少运行：

```sh
cd "$GOROOT/src"
go test cmd/llvmtoolexec cmd/go/internal/work cmd/dist

cd /path/to/llvm-project
llvm/cmake-build-debug/bin/llvm-lit -sv \
  llvm/test/CodeGen/AArch64/goobj-ir-config.ll
```

此外应以一个简单 `main` package 执行真实的 `go build -toolexec`，并运行产物。
这同时验证 compiler archive、IR metadata、`llc`、GoObj archive member 和
`cmd/link` 的接口。

## 回归测试机制

GoALLC 不维护一套与 Go 仓库重复的测试源码。LLVM 测试由
`cmd/internal/testdir` 发现并复用 `$GOROOT/test` 中已有的测试：

- codegen 候选是 `test/codegen` 下 recipe 为 `// asmcheck` 的文件；
- runtime 候选是 testdir 原本扫描目录中 recipe 为 `// run` 的文件；
- `test/llvm_tests.json` 分别维护 codegen 和 runtime 的白名单与黑名单；
- codegen source 出现 `// LLVM-OPT` directive 时，runner 还会执行
  `opt -passes=default<O2>`，并用 `LLVM-OPT` prefix 检查优化后的 IR；
- 白名单是当前必须通过的用例；黑名单支持 glob，用于记录尚未覆盖的范围；
  精确白名单优先于宽泛黑名单；
- runner 会拒绝拼错的白名单、无匹配项的黑名单，以及未被任一名单分类的
  候选，并报告当前白名单文件数和候选文件总数。

### LLVM IR codegen 检查

codegen 白名单文件直接在原 Go 源码中使用 LLVM `FileCheck` 指令，例如：

```go
// LLVM-DAG: define goabiinternal { i64, i8 } @codegen.div_ndivis6_int64
// LLVM-DAG: sdiv i64
// LLVM-DAG: srem i64
```

runner 使用 `compile -enablellvm -llvmironly` 生成 `.ll`，然后执行：

```sh
FileCheck --check-prefix=LLVM test/codegen/example.go < package.a.ll
```

因此 codegen 测试只检查 LLVM IR，不调用 `llc`、不链接也不运行。可使用
`LLVM:`、`LLVM-LABEL:`、`LLVM-DAG:`、`LLVM-NOT:` 等标准 FileCheck
指令。原有 asmcheck parser 会忽略这些独立注释行，社区 Go 的机器码检查仍
按原方式运行。每个 codegen 白名单文件至少要包含一条 `// LLVM...`
指令。

`FileCheck` 的选择顺序为 `GOALLC_FILECHECK`、
`$GOALLC_LLVM_DIR/bin/FileCheck`、`$GOROOT/llvm/bin/FileCheck`。

### LLVM runtime 检查

runtime 白名单不增加新的 recipe，也不复制测试源码。runner 仍解析原文件的
`// run` 参数、build constraint、超时和期望输出，但构建步骤改为用
`-gcflags=-enablellvm` 选择命令行 `main` package，再通过 `cmd/llvmtoolexec`
替换其对象：

```text
原有 // run 用例
  -> go build -gcflags=-enablellvm -toolexec="llvmtoolexec ..."
  -> compile -enablellvm -llvmironly
  -> llc 生成 _go_.o
  -> cmd/link
  -> 运行并沿用原 testdir 输出检查
```

`llc` 的选择顺序为 `GOALLC_LLC`、`$GOALLC_LLVM_DIR/bin/llc`、
`$GOROOT/llvm/bin/llc`。

### 扩大覆盖范围

新增覆盖时：

1. 先确认整个现有测试文件都能由当前 LLVM lowering 处理；
2. codegen 文件在源码中增加针对 LLVM IR 的 FileCheck 指令；
3. 在 `test/llvm_tests.json` 中增加精确白名单项和简短能力说明；
4. runtime 文件只需加入白名单，不修改其原有 `// run` recipe；
5. 运行 LLVM 定向测试，并同时运行对应的原生 asmcheck/run 测试。

定向运行全部 LLVM 白名单：

```sh
go test cmd/internal/testdir -run='^Test$/^LLVM$' -v
```

只运行一个 LLVM codegen 或 runtime 子测试时，可继续在 `-run` 中追加
对应的 slash-separated subtest 名称。黑名单用于明确尚未覆盖的范围，不应
把实际失败的用例加入白名单后再按 expected failure 处理。

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
