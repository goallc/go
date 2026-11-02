# GoALLC：LLVM IR 到 GoObj 的开发链路

本文记录 Go SSA 到 LLVM lowering 的端到端链路。默认路径在 `cmd/compile`
进程内完成 LLVM IR 优化、GoALLC pre-codegen pipeline 和 GoObj 生成，再复用
compiler 原有的 Go package archive writer；不会把 IR 写成文本，也不启动
`opt`、`llc` 或 `toolexec`。LLVM 动态/静态链接只改变 compiler 如何链接同一
payload，不改变这条运行时数据流。

正常非 bootstrap compiler 默认选择 LLVM；原生对照需要显式
`-enablellvm=false`。默认选择不意味着所有平台或模式都已获得兼容性资格。
总体架构与实现边界见 [../GOALLC.md](../GOALLC.md)。

## 数据流

```text
go build -gcflags=all=-enablellvm
  -> compile 构造内存中的 LLVM Module
  -> early IR 准备（CPU/FMV、write-barrier record 属性）
  -> Module::RunPasses（默认 default<O2>，-N 使用 default<O0>）
  -> pre-codegen callback：写屏障展开、模块准备、注册 target passes
  -> TargetMachine::EmitToMemoryBuffer
       -> 标准 CodeGen IR 准备
       -> late PreISel：逐函数 statepoint / relocation SSA
       -> 指令选择、寄存器分配、Go 栈帧与机器码、GoObj
  -> compiler archive writer 写入 __.PKGDEF 和唯一的 _go_.o
  -> cmd/link 读取 __.PKGDEF 和 _go_.o
```

`-enablellvm` 现在选择进程内 LLVM backend。`__.PKGDEF` 仍完全由 Go compiler
生成并位于 archive 的第一个成员；LLVM 返回的内存 object 直接成为唯一的
`_go_.o`。`-linkobj` 拆分输出也沿用原 compiler 语义。

`-llvm-keep-ir` 只控制诊断输出：进程内 backend 仍完整生成 GoObj，同时保留
`<archive>.raw.ll`（前端快照）、`<archive>.ll`（early IR 准备后）和
`<archive>.opt.ll`（IR 优化后、late statepoint 重写前）。原生 backend
对照使用一次显式 `-enablellvm=false` 的独立构建。

## 类型描述符和只读数据

进程内 backend 不会跳过 compiler 的 `dumpdata` 准备阶段。`reflectdata` 仍按
原生 compiler 的方式在 `base.Ctxt.Data` 中生成最终的 `obj.LSym` 布局；随后
LLVM lowering 收集 `base.Ctxt.Data` 中受支持 symbol kind 的定义，以及 SSA/data
lowerer 记录的额外 roots，沿 `Gotype` 和 relocation 收集数据闭包，再生成 LLVM
globals。这包括类型描述符，也包括普通静态数据。descriptor 的主体由同一份
`rttype` runtime ABI layout 构造为 packed
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
`!goobj.symbol.flags` 显式保存 GoType、Itab、typelink、ReflectMethod、UsedInIface、linkname
以及 Local+DUPOK 重叠等没有等价 LLVM 表示的位，不依赖 LLVM 根据 struct type
名字推断 GoType 或 Itab。
普通 `R_ADDR` 由 LLVM initializer 和 target symbol 语义直接表达，不向
metadata 复制同一份信息。`R_ADDROFF` 是 GoObj 的 32 位 section offset；
`R_METHODOFF` 还额外控制 Go linker 的 dead-method elimination。LLVM 原生
relocation 均没有这两种对象格式语义，因此用 `!goobj.relocs` 精确标出它们的
relocation offset 和完整类型，包括 `R_WEAKADDR`、`R_WEAKADDROFF` 的
`R_WEAK` 位，不再使用单独的 weak-offset 表。这避免让通用 LLVM backend
依赖 GoType flag、target symbol 命名、`%go.runtime.Method` 的名字或字段布局
来猜测 offset relocation。LLVM
initializer 始终是地址关系的 source of truth，metadata 只承载对象格式特有的
剩余语义。零宽度的 linker
保活边 `R_KEEP` 不伪装为地址常量，而以 `!goobj.keep !{ptr @target, ...}`
附着到源定义。`gotype` aux 同样是定义上的 `!goobj.gotype !{ptr @type}`；
数据的 linker marker 使用 `!goobj.marker_relocs !{!entry, ...}`，每个 entry
只保存 `{ptr @target, i32 relocation_type, i64 addend}`。source 由所附着的定义
提供，不再在模块级关系表中重复记录。因此 LLVM 链接选择强定义或 weak/DUPOK
定义时，关系随同该定义选择，不会合并已丢弃定义的保活边和初始化顺序。

关系中的 target 是 LLVM global/function 的直接引用，重命名和 RAUW 会同步
更新。关系涉及的值仍进入 `@llvm.compiler.used`，防止 GlobalDCE 删除仅承担
对象格式语义的值；这个特殊全局不生成数据，也不代替 Go linker 的可达性判断。
writer 从最终定义的 attachment 恢复 GoObj relocation/aux。LLVM 和 Go producer
需要一起更新；旧的模块级 keep/marker 表会被明确拒绝。最终链路仍只有
`__.PKGDEF + _go_.o` 两个有意义的 archive members；不会生成或合并 native data
object。

可移除的 map initializer 调用同样以 `!goobj.weak_calls !{ptr @callee, ...}`
附着在所属 package init 定义上；已丢弃定义的调用边不会污染胜出的定义。
只有最终机器码中该 caller 到目标的直接调用被标为 weak，取地址和其他 caller
的调用仍为 strong。outlined callee 保持 noinline；caller 被克隆时 attachment
随定义复制，被内联到其他函数时不扩大 weak 契约，调用保守地保持 strong。
旧的模块级 weak-call 表会被明确拒绝。

静态 interface conversion 还会把带有 `ItabInfo` 的 roots 纳入同一数据闭包。
itab 使用 `%go.runtime.ITab` 和按实际方法数扩展的 packed LLVM struct 表达；
固定的 `Fun[0]` 后面按 LSym 的最终大小追加 `ptr` 数组。`ptr` 与 runtime
`uintptr` 具有相同大小和对齐，因此单方法和多方法 itab 仍保持连续的 ABI
布局，同时 LLVM 能保留静态方法入口的 function-pointer 语义。方法入口仍保留
weak `R_ADDR` relocation。interface 方法调用从 itab 槽直接加载 LLVM pointer，
并以原 SSA `AuxCall` 的 ABIInternal signature 发出 indirect call。

Go linker 的 dead-method elimination 还依赖函数上的零宽度
`R_USEIFACE`、`R_USEIFACEMETHOD` 和 `R_USENAMEDMETHOD`。它们不对应 LLVM
机器指令或地址常量，因此 compiler 先在 `llvm.sideeffect` 指令上保存
`!goobj.marker_reloc` 的 target、type、addend。这些指令随内联复制到调用者；
优化结束后，plugin 将记录附着为最终所在函数的 `!goobj.marker_relocs`，再移除
指令。GoObj writer 恢复该函数的 relocation 列表，普通 LLVM call graph 不能
替代这项 linker 契约。不能在内联前把函数 marker 仅存成函数 attachment。

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

数据 lowering 已不局限于 type-rooted closure；支持的 symbol kinds 由
`llvmDataSymbolKindSupported` 明确列出。type descriptor 中未被 schema 覆盖的
尾部和其他 generic data 仍使用 bytes/relocation layout。新增 schema、symbol kind
或 root 必须明确其 relocation、GC 和 linker 契约，并增加对应 GoObj regression。

## 闭包调用 ABI

Go SSA 的 `ClosureCall` / `ClosureLECall` 不能按普通 indirect call lowering。
调用的第一个 SSA operand 是从 funcval 首字加载的 code pointer，第二个 operand
是 funcval 自身；后者不是普通 Go 参数，而是 ABIInternal 的隐藏 closure context。
LLVM lowering 在 `AuxCall` 描述的普通参数之后追加一个 `ptr nest` 参数，并在
call site 和被调函数定义上保持相同签名。GoObj target 把该参数固定放入原生
REGCTXT：amd64 使用 RDX，arm64 使用 X26，不把它计入普通参数寄存器或栈参数布局。

函数需要 context 的条件是 `NeedCtxt()` 或存活的 SSA `OpGetClosurePtr`，不是两者
必须同时存在：即使 deadcode 删除了 context 读取，`NeedCtxt()` 仍保留调用契约。
closure context 只支持 ABIInternal；不合法的 callee/context 类型或 ABI 被明确拒绝。
ABI0 的普通 static call 仍使用独立 calling convention，但当前没有
安全的 ABI0 closure-context 契约。结果沿用 `AuxCall` 的逻辑次序：寄存器结果
通过直接 LLVM return（必要时 tuple）返回，内存结果通过 typed `goret` 参数和
`goretindex` 对应 caller-owned homes。`OpSelectN` 取回相应结果；不能把所有
多返回值都解释成 LLVM aggregate return。

nil funcval 调用必须在 indirect call 前产生可恢复的 Go panic。原生 Go backend
依赖 funcval 首字的机器 load 在零地址 fault；普通 LLVM null load 则允许优化器
按 undefined behavior 处理。包含 closure code load 的 LLVM caller 因此带有
`null_pointer_is_valid` function attribute，真实 code pointer 保持单次普通
`load ptr`。load 结果直接进入 indirect call，不会成为 dead load；attribute
则阻止 LLVM 仅因 funcval 可能为 nil 就把该路径改写为 `unreachable`。最终
机器代码与原生 Go backend 一样，依赖零页访问产生 fault。

通用 `OpNilCheck` 先用临时 marker 保持 SSA 翻译顺序，再在 CFG/PHI 完整后
展开为 `p == nil` 分支、`runtime.panicmem` 调用和 `unreachable`。分支附带
`!make.implicit !{!"goallc"}`。目标相关的 ImplicitNullChecks 仅在非负首个
内存页偏移、访存依赖和调度等条件成立时，将其折叠为 Go signal 路径可处理的
同步 fault；否则保留显式 panic 路径。panicmem 仍是参与 statepoint 的普通
非 leaf 调用。替换指令保留 nilcheck 的 DILocation，包括 inlined-at 链。
实现见 `src/cmd/compile/internal/ssa/ssa2llvm_nilcheck.go`。

带 closure context 的 GoObj 函数发生 stack growth 时必须调用
`runtime.morestack`，使 REGCTXT 在 slow path 中保留；普通函数仍调用
`runtime.morestack_noctxt`。LLVM backend 根据 `nest` 参数选择这两个入口。
这一规则与调用点的 hidden context lowering 是同一 ABI 契约，不能只修改其中
一侧。

target prologue emission 将 morestack 重试边建模为机器 CFG loop。无 profile
时，compiler 内的 `llvmCodeGenOptions` 固定设置 `-force-loop-cold-block`，让
`MachineBlockPlacement` 按 target 已设置的极低 slow-path 边概率把 morestack
block 移到正常函数体和 `RET` 之后。这样热路径从 stack check 直接 fallthrough 到函数体，与原生 Go 的扩栈
序言布局一致；不能删除该参数后仅依赖初始 MBB 顺序。

## 固定栈槽和动态 alloca

每个仍有 use 的 SSA `OpLocalAddr` 在 LLVM entry block 预先建立唯一的固定
`alloca`。栈槽按源变量身份和位置复用，并使用 Go 类型要求的对齐；循环或分支
中的 `OpLocalAddr` 只引用该 entry-block 栈槽，不在控制流内部重复分配。没有
use 的 local 不生成 LLVM 栈槽，避免无意义地增大 frame。

GoObj stack growth、SP 恢复和当前 stack-map 路径只支持编译期固定 frame。
因此 amd64 和 arm64 GoObj target 对 variable-sized `alloca` 都确定性报错。
固定存储的原始 base 是 frame identity，不生成普通 `gc.relocate`；同对象派生
地址规范化为 base + integer offset，在实际使用处重建。typed `byval` 参数和
`goret` 结果 homes 使用同一固定 frame 规则。

插件对优化后的固定对象进行 pointer-content liveness 分析；活跃内容进入
Args/LocalsPointerMaps，地址可观察的对象另生成函数级 StackObjects，并通过
`llvm.stackcoloring.no_merge` 保留 frame identity。对象身份不意味着每个调用点
内容都活跃。若活跃区间的 producer 未能在 safepoint 前初始化所有 pointer slots，
插件在相应起点插入 inline zero initialization。

相关回归包括 `test/llvm_closure.go`、`test/llvm_alloca_statepoint_gc.go`、
`test/abi/llvm_args_results.go` 及 `src/cmd/llvmplugin/testdata` 中的固定 frame、
aggregate、liveness 和 relocation 用例。支持边界见
[plugin README](../src/cmd/llvmplugin/README.md)；不沿用早期 whitelist 计数推断
当前运行时资格。

## LLVM IR 契约

LLVM IR 本身是 frontend 到 `llc` 的唯一配置载体。`compile` 设置 GoObj
target triple，并将公共兼容性约束与每包身份分开：

```llvm
target triple = "aarch64-apple-darwin-goobj"
!llvm.module.flags = !{!0}
!0 = !{i32 1, !"goobj.abi", !1} ; Error: incompatible inputs cannot link
!1 = !{!"goallc.goobj", !"darwin", !"arm64", !"go1.27",
       !"GOARM64", !"v8.0", !"0", !2}
!2 = !{!"regabiwrappers", !"regabiargs"}
!goobj.config = !{!3}
!3 = !{!"build-id", !"main", !"1", !"0", !"0000000000000000"}
```

`goobj.abi` 的八个字段依次是 schema、GOOS、GOARCH、Go compiler 版本、
架构设置 key/value、shared 标志和 experiment node。LLVM 原生 module flag
合并规则检查整个节点是否一致，不需要 LTO 自己逐字段比较。

`!goobj.config` 的五个字段依次是 build ID、package prefix、main 标志、std
标志和十六进制 package fingerprint。它与 imports 一样仍属于原始输入包；
LTO 必须保存包身份与依赖图，不能把这些记录简单合并成 main 的属性。

`codegen::configureGoObjFromModule` 是 `llc` 与进程内 binding 共用的解析入口，
在创建 writer 前校验配置，只读取上述格式。没有配置的手写 GoObj IR 仍可使用
`-goobj-*` 选项。

普通编译直接将 `!goobj.gotype !{ptr @type.T}` 附在定义上；LLVM 合并时随胜出
定义保留类型引用。GoObj writer 直接读取定义上的 attachment。NoSplit 从已有的
`go-nosplit` 函数属性生成，不再重复写入 symbol flags。
DWARF 编码版本由标准 `Dwarf Version` module flag 指定，开启程度由各个
`DICompileUnit.emissionKind` 指定，不再生成 `goobj.debug.config`。
每个 Go CU 的 `file.filename` 是原始 import path；源包的 `DIModule` 保存真实
package 名，并使用同一 file。CU 的 imported-module 关系将它保留在原生 debug
图中，空包也不会丢失包名。GoObj 为各个 CU 生成对应的 package-name carrier，
同路径重复记录必须同名，输出按 package prefix 排序。函数和变量仍使用真实源文件。

变量从 `DISubprogram.retainedNodes` 和原生 debug records 收集，不再生成
`goobj.debug.vars`。所有 `DIType`（包括数组）携带精确 Go 类型名。
`DILocalVariable.annotations` 只保存非默认的 Go 语义：`go.return = "1"`
标记返回值，`go.dictindex` 保存十进制的非零字典索引（最大 65535）。这些信息
随原生变量节点经过克隆和内联，无需另一张模块级变量表。

对应 LLVM 测试为
`llvm/test/CodeGen/AArch64/goobj-ir-config.ll`。LLVM 侧实现位于
`llvm/lib/CodeGen/CommandFlags.cpp`、`llvm/lib/CodeGen/CodeGenTargetMachineImpl.cpp`
和 `llvm/lib/MC/GoObjObjectWriter.cpp`；进程内解码入口位于 vendored
`go-llvm/SupportBindings.cpp`。

## 集成与调试边界

函数定义的 subprogram 到 symbol 映射直接取自 LLVM `!dbg`，包括优化器和 FMV
产生的克隆。`!goobj.debug.funcs` 只补充无法从定义恢复的抽象内联实例：同一个
storage symbol 可以对应多个不同的 `DISubprogram`。省去重复映射不改变保活契约；
前端和 FMV 仍将 debug 所需的函数放入 `@llvm.compiler.used`，避免 GlobalDCE
删除仍被变量或内联记录引用的定义。

GoObj variable locations are emitted from final machine-frame offsets and
LLVM debug-value histories as Go linker-native DWARF4/5 location lists.
Heap-promoted variables with a canonical heap-pointer home use an additional
dereference. Register clobbers, unavailable values, and fragment boundaries
limit each location's PC range. Variables without a frontend location,
inlined-variable instances, and unsupported expressions remain unavailable;
the backend does not infer locations from source names or ABI offsets.

package 选择由 cmd/go 已有的 `-gcflags` 规则完成。无 pattern 的 flag 只覆盖
命令行 package，但未覆盖的 package 仍默认 LLVM。只让一个包使用 LLVM 时，
应先设置 `-gcflags=all=-enablellvm=false`，再用精确 pattern 覆盖目标包。

compiler 所使用的 LLVM binding、动态 `libLLVM`（动态模式）和静态 archive
（静态模式）必须来自 `make.bash` 记录的同一 payload；pass plugin 必须由
`make.bash` 使用该 payload 的 headers/CMake package 构建，并安装在 Go
toolchain 自己的 `pkg/tool/<host>/lib` 中，动态 `libLLVM` 也安装在同一目录。
`-llvm-keep-ir` 提供优化前后 IR；GoObj、链接器和机器码检查由统一 testdir 的
`test/codegen` 驱动完成。外部 `opt`/`llc` 仍可用于 LLVM 仓库自身的格式级测试，
但不再是 Go toolchain 的集成入口，也不参与 Go action cache identity。

插件源码和测试位于 Go 仓库 `src/cmd/llvmplugin`。LLVM 提供通用插件回调、
target-pass 扩展点和 GoObj 对 Machine StackMaps 的适配。正式流水线由
`GoALLCStatepointPlugin.cpp` 编排：

1. IR 优化前，配置 write-barrier records 并执行 CPU/FMV early IR 准备。
2. pre-codegen callback 在优化后展开 write-barrier records，完成模块级 marker
   准备，并注册 target passes。
3. 标准 CodeGen IR 准备之后、指令选择之前，`GoALLCPreISelPass` 逐函数调用
   `rewriteStatepoints`。callback 中直接重写完整 module 只用于 emit-IR 测试模式。
4. LLVM 完成指令选择、寄存器分配和 frame lowering；Go-owned read-only callback
   根据最终机器指令标记异步抢占保护区，LLVM 将标签转换为最终 PC 范围。

前端给 Go ABI 函数声明 `gc "goallc"`；插件注册 GC strategy，并消费前端契约。
普通调用根据 CFG 逆向活跃性生成 `gc.statepoint` / `gc.result` / `gc.relocate`。
支持的 struct/array SSA aggregate 在定义旁提取 pointer leaves，在 use 处用当前
值重建；固定 pointer vector 保留同类型 root。`gc-leaf-function` 按调用点分类，
定义带该属性不代表函数体内所有调用被豁免。符合 Go ABI 的 `musttail` 没有
caller continuation，不作为 caller safepoint；一般 `invoke` / 非 leaf `callbr`
等不支持形态仍拒绝。`llvm.go.defer.edge` 是单独定义的零指令控制流契约。

固定 alloca / byval / goret 的 base 保留为 FrameIndex 身份，地址在 use 处重建，
不参与普通 relocation SSA；其内容由独立的 typed layout/liveness 描述。从这些
homes 中加载的普通 pointer values 仍参与普通 root tracking。普通指针的 relocate
定义通过临时 promotable alloca 与 `PromoteMemToReg` 构造 PHI，覆盖合流与循环。
详细协议见 [plugin README](../src/cmd/llvmplugin/README.md)。

GoObj AsmPrinter 适配标准 Machine StackMaps，保留 deopt 布局和 GC locations
到 MCContext。writer 在最终 layout 后解释 SP 位置，生成 pointer maps 与
PCDATA。`Indirect [SP+offset]` 表示槽内存有 pointer：当前 frame 内对应 locals，
caller-owned 参数/结果区对应 args。`Direct SP+offset` 是地址本身，不能当作该槽
中的 pointer bit；在已验证的 fixed-object record 内，它才是展开内容 bitmap 的基址。

LLVM formal-argument lowering 从存活函数的参数类型与 Go ABI 布局推导入口参数
homes，以零字节 `EntryArgsStackMapID` 保存函数级入口图。pair 0 是 EntryArgs 与
empty locals，`PCDATA_StackMapIndex` 从 0 开始。PCSP 回到入口栈深时选择 pair 0；
普通 statepoint 在 CALL 起点选择实际活跃 pair，覆盖同 PC 的入口深度转换。
`morestack` 是 frame lowering 插入的普通 ABI0 MIR call，不再制造专门的
stack-growth statepoint。

writer 按完整 `(Args, Locals)` pair 去重。精确匹配的栈输入 pointer 可复用
canonical ABI home，`gc.relocate` 从同一位置 reload；无法证明的普通值使用正常
statepoint spill。活跃 fixed-object 内容也可按 typed bitmap 扩展到 args 或 locals。
地址可观察的 alloca 另外产生 `FUNCDATA_StackObjects`。`PCDATA_ArgLiveIndex`
从最终机器 CFG 已存在的 home stores 推导可见性，不为 traceback 凭空添加 spills，
也不替代 GC 的 pointer maps。

GoObj 依次写出索引 0 的 `PCDATA_UnsafePoint` 和索引 1 的
`PCDATA_StackMapIndex`。UnsafePoint 并非恒为 safe：普通支持形态可细化为区间，
写屏障、stack check/morestack、inline asm 与 arm64 REGTMP 活跃范围等受保护。
runtime、reflect、nosplit、systemstack、显式 whole-function 策略和无法识别的
协议保守保持全函数 unsafe。

AArch64 GoObj 使用 Go 栈链：LR 在 `SP+0`，当前函数为未来 callee 写入的 FP link
在 `SP-8`，caller 已写入的 FP link 在 `SP+StackSize-8`。writer 由最终物理
`StackSize` 推导 `_func.locals=StackSize-8`，locals bitmap 范围是
`[SP+8, SP+StackSize-8)`。小 frame 通过 pre-index 保存 LR 并更新 SP；大 frame
先保存 frame record 再切换 SP，避免异步 traceback 观察半构造的 frame。

`make.bash` 会使用同一个 LLVM payload 的 CMake config 构建插件，并以新 inode
原子安装到 Go toolchain 自己的目录。规范的 LLVM payload、plugin、Go 构建和验证流程见
[goallc-build.md](goallc-build.md)：

```sh
cd "$GOROOT/src"
./make.bash \
  -llvm-dir=/path/to/goallc-llvm \
  -llvm-version=23 \
  -llvm-link=dynamic
```

安装结果为 Darwin 上的
`$GOROOT/pkg/tool/<host>/lib/GoALLCStatepoints.dylib` 或 Linux 上的
`$GOROOT/pkg/tool/<host>/lib/GoALLCStatepoints.so`，动态 `libLLVM` 同目录安装。
静态 archive 保留在 `pkg/goallc-llvmplugin/lib`。LLVM payload 内不安装、不查找 plugin。不要用其他 LLVM 安装构建后
再复制产物；pass plugin 的 C++ ABI 必须和 compiler 所链接及 `llc` 所使用的
LLVM payload 精确匹配。

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

相关 compiler 参数如下：

| 参数 | 行为 |
| --- | --- |
| `-enablellvm` | 正常 compiler 的默认值；选择进程内 LLVM backend |
| `-enablellvm=false` | 选择原生 Go backend |
| `-llvm-opt-passes` | Default `auto`: `default<O0>` with `-N`, otherwise `default<O2>`. An explicit pipeline overrides IR optimization selection; `none` skips IR optimization. |
| `-llvm-keep-ir` | 保留 `.raw.ll`、准备后 `.ll` 和优化后 `.opt.ll` |
| `-llvm-lto` | 输出 pre-link O2 后的 `__.LLVMBC`；当前主线 linker 尚不能消费，见下节 |
| `-gcflags` package pattern | 覆盖匹配包的 flags；未覆盖的包仍默认 LLVM |

`-N` also disables machine-code optimization, independently of an explicit IR pipeline. `-l` disables both Go and LLVM inlining. Required ABI, GC/statepoint, and object emission passes still run at O0. The runtime package retains Go's existing exception to `-N`.

例如保留进程内优化前后的 IR：

```sh
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm -llvm-keep-ir' \
  .
```

例如只让一个依赖走 LLVM，可使用：

```sh
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm=false' \
  -gcflags='example.com/project/internal/foo=-enablellvm' \
  .
```

反过来，若大部分 package 走 LLVM、仅 runtime 包保留原生 backend，可利用
`-gcflags` 的“最后一个匹配规则生效”语义：

```sh
"$GOROOT/bin/go" build \
  -gcflags='all=-enablellvm' \
  -gcflags='runtime=-enablellvm=false' \
  .
```

开发时至少运行：

```sh
cd "$GOROOT/src"
go test cmd/internal/llvmbackend cmd/go/internal/work cmd/dist
go test cmd/internal/testdir -run '^Test$/^codegen$'

cd /path/to/llvm-project
llvm/cmake-build-debug/bin/llvm-lit -sv \
  llvm/test/CodeGen/AArch64/goobj-ir-config.ll
```

此外应以一个简单 `main` package 执行进程内 `go build` 并运行产物，验证内存
IR、进程内 plugin/codegen、compiler archive writer 和 linker。动态/静态两种
链接配置使用相同测试，不建立第二套运行入口。

## LTO 编译输出与链接边界

`-llvm-lto` 执行 early IR 准备，再运行 LLVM 原生 `lto-pre-link<O2>`，生成
`__.PKGDEF + __.LLVMBC`。导出数据仍可用于后续包编译；pre-codegen、late
statepoints 和机器码生成留到链接阶段。`.opt.ll` 对应写入包内的 pre-link 优化结果。

截至 2026-11-02，主线 `cmd/link` 明确拒绝 bitcode archive。端到端
`go build -lto=full`、whole-program `lto<O2>`、单一物理 GoObj 与逻辑包恢复在
尚未合并的 [PR #347](https://github.com/goallc/go/pull/347) 中。主线 compiler
的 bitcode 支持不能独立视为完整 LTO 使用入口。

## 标准库的分层构建和测试

扩大到标准库时必须记录 LLVM 实际覆盖的依赖范围。以下三种命令不能混称为
“标准库使用 LLVM”。每次使用新的 `GOCACHE`，避免上一层生成的 archive 掩盖
本层的编译边界；`-x -work` 和 compiler 的 `-llvm-keep-ir` 可在调查失败时追加。
LLVM 已是默认后端，单包与部分依赖测试必须先指定
`-gcflags=all=-enablellvm=false`，再覆盖选定包。原生对照使用
`-gcflags=all=-enablellvm=false`；只省略 flag 不会回到原生后端。

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
  -gcflags='all=-enablellvm=false' \
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

LLVM_GCFLAGS=('-gcflags=all=-enablellvm=false')
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

### all.bash CI 策略

LLVM 是默认后端；CI 在 amd64 和 arm64 上分别运行一次 `make.bash`，共享 SDK 后
分片运行 `run.bash --no-rebuild`，覆盖 `all.bash` 的标准库、命令、dist 特殊模式及 testdir。
已知失败按具体用例、模式或 TestMain 构建失败的包记录在
`test/llvm_known_failures.json`，每项附原因；用例排除可限定架构，通过原生 `-skip`
机制实现。未列出的失败会使 CI 失败。重新评估时设置
`GOALLC_TEST_KNOWN_FAILURES=0`，详见 [构建说明](goallc-build.md)。

### 历史资格记录

2026-08-11 的 Darwin/arm64 开发载荷和 Linux/amd64 v8 复测是有日期、revision
和 package scope 的历史记录，不代表当前失败清单或平台资格。原始记录见
[此前文档快照](https://github.com/goallc/go/blob/42dc37cc80160e792a97df75221af57263fc6a2f/doc/goallc-llvm-goobj.md#darwinarm64-开发载荷阶段结果)。
后端默认值、X86 ABI 状态修复、stack-map 协议和测试入口均已继续演进；复测应使用
上面的显式选择命令和当前 `test/llvm_known_failures.json`，不要复制历史 whitelist
数字或失败症状作为当前结论。

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

GoALLC 复用 `cmd/internal/testdir` 的原生 `Test` 入口、目录扫描和全部 recipe。
不再按 recipe 类型创建 LLVM 候选集合，避免漏掉 `compile` 和 `errorcheck` 等回归。
`asmcheck` 运行 LLVM codegen 并保留以下 FileCheck 检查；需要原生汇编对照时使用
`GO_GCFLAGS=-enablellvm=false go test cmd/internal/testdir -llvm_codegen=false`。

### LLVM IR codegen 检查

codegen 候选文件直接在原 Go 源码中使用 LLVM `FileCheck` 指令，例如：

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

这条命令已经在 compiler 内完成优化、callback 和 codegen。`LLVM:` 和
`LLVM-OPT:` 分别检查优化前后 IR，`LLVM-ASM:` 检查 object 反汇编；迁移自原
外部入口的生成物契约还可使用 `LLVM-OBJVIEW:`、`LLVM-OBJSUMMARY:`、
`LLVM-NATIVE-OBJSUMMARY:`、`LLVM-NM:` 和 `LLVM-LINK:` 检查 GoObj 文本/规范化
JSON、与原生 object 的关键字段一致性、符号表以及最终链接器输出。`LLVM-LINK:`
成功后还会运行生成的 executable。所有前缀都支持 `LABEL`、`DAG`、`NOT`、
`COUNT` 等标准 FileCheck 形式和 AMD64/ARM64 架构后缀。原有 asmcheck parser 会
忽略这些独立注释行，社区 Go 的机器码检查仍按原方式运行。

`FileCheck` 的选择顺序为 `GOALLC_FILECHECK`、
`$GOALLC_LLVM_DIR/bin/FileCheck`、`$GOROOT/llvm/bin/FileCheck`。

### 执行与定向复现

`run`、`compile`、`rundir` 等保持原 recipe 的参数、构建约束和输出断言。
默认后端保证直接 `go tool compile`、`go run` 及其完整依赖都使用 LLVM；不再
注入 `-enablellvm`、`-w` 或 `-llvm-package-only` 来改变执行边界。

```sh
# 直接复现一个用例；不应用 dist 的已知失败名单。
go test cmd/internal/testdir -run='^Test$/^fixedbugs$/^issue77534.go$' -v
# 使用 CI 相同的名单和超时策略。
cd src && ./all.bash -llvm-dir=/path/to/llvm-payload
```

新增 recipe 默认参与测试。已知不支持或异常慢的用例应在集中名单中记录具体名称
和原因；修复后删除条目，不修改其原有断言来制造通过。

## 当前范围与后续工作

- 当前 compiler target triple 覆盖 Darwin/arm64、Linux/amd64 和 Linux/arm64；
  不能由 LLVM 支持其他 CPU 推断 GoALLC 已支持对应平台。
- closure、interface、defer、pointer maps、StackObjects、异步抢占和 DWARF 已有
  实现与回归。实现存在不等于全部组合获得运行时资格；按具体用例和当前测试记录判断。
- 普通 LLVM package 由进程内 backend 生成完整 `_go_.o`，export data 由 Go writer
  生成；不合并另一份原生 compiler 数据对象。汇编/native archive members 仍按其
  正常协议处理。LTO 的 `__.LLVMBC` 是另一个明确的输出模式。
- 动态/realigned frame、scalable pointer vectors、通用 moving-heap base/derived
  推断及任意 invoke/EH 形态不在现有协议范围；不支持输入应被明确拒绝。
- 支持的静态 data kinds 和额外 roots 的闭包已有 lowering；类型/itab 可用已知
  schema，没有 schema 的内容保留 bytes/relocations。新增 root 必须验证相应契约。
- 新增 GoObj metadata 时优先使用 LLVM 原生类型、initializer、linkage、module flags
  和 debug graph；仅为没有原生表达的 GoObj 剩余语义定义 attachment。
