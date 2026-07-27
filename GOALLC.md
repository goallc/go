# GoALLC 项目现状、架构与后续基线

更新日期：2026-07-27

> 2026-07-27 更新：`compile -enablellvm -llvmironly`、`cmd/llvmtoolexec`
> 和 LLVM 的 GoObj metadata consumer 已经打通一条仅面向简单 package 的
> 自动链路。具体的输入/输出契约、运行方法和当前限制见
> [doc/goallc-llvm-goobj.md](doc/goallc-llvm-goobj.md)。本文后续章节保留了
> 此前 sidecar 原型的背景和升级记录。

## 1. 项目目标

GoALLC 的目标是在尽量保持社区 Go 前端、语言语义、运行时、包格式和
工具链兼容性的前提下，以 LLVM 作为 Go 编译器的可选后端。

当前工作已经覆盖了这条链路中的三个层次：

1. 在社区 Go 编译器中引入 Go LLVM binding，并实现一版 Go SSA 到
   LLVM IR 的原型 lowering。
2. 在 LLVM 中实现 Go ABI 和 GoObj（Go gc 工具链对象格式）的初步支持。
3. 通过 `go build -toolexec` 把 LLVM IR 生成的 GoObj 追加到 Go package
   archive，并交给社区 Go linker 完成链接。

当前成果是一个有真实端到端验证的原型，但还不是可以替代社区 Go 后端的
完整编译器。特别需要区分：

- Go SSA lowering 分支目前输出 LLVM IR sidecar，同时仍继续运行原生 Go
  后端；
- LLVM 的端到端 GoObj 测试目前主要消费手写 `.ll`；
- 两条路径尚未接成“Go SSA -> LLVM IR -> LLVM CodeGen -> GoObj -> Go
  archive”的自动生产链路。

## 2. 仓库与分支基线

### 2.1 `go`

路径：`/Volumes/Disk1/00.Work/00.Code/goallc/go`

Remote：

- `origin`: `https://github.com/goallc/go`
- `personal`: `git@github.com:zhouguangyuan0718/go.git`

当前检出分支：

- 正式集成分支 `go1.27.master`
- 社区基线 `official/release-branch.go1.27`
- 当前社区基线提交 `b7cc93a369`，版本为 `go1.27rc2`
- 原 `personal/go1.25.master` 的 6 笔提交已经重放到该基线上：vendor
  `go-llvm`、初版 SSA lowering 和 `cmd/objview`
- GitHub ruleset 继续禁止删除和 non-fast-forward 更新；已移除与官方 Go
  历史不兼容的 linear-history 限制，因为 release branch 本身包含 merge
  commit

另有一个更新的 SSA lowering 实验分支：

- `personal/codex/add-llvm-backend-support-for-ssa-ops`
- HEAD `8b64b9654f`
- 直接基于 `go1.25.4`，领先 9 笔提交
- `personal/go1.25.master` 停在 `2650cf0dfc`，包含前 6 笔提交；
  主题分支继续增加 3 笔 lowering 提交

9 笔提交按顺序为：

1. `13e5c6c4d2`：vendor `go-llvm` 并支持构建
2. `2da898daf0`：初版 Go SSA -> LLVM IR
3. `77b20d0f59`：扩展 SSA lowering
4. `17c622fede`：新增 `cmd/objview`
5. `7211cd1ddc`：扩展 GoObj section dump
6. `2650cf0dfc`：继续扩展 GoObj section dump
7. `e69c3f5ffc`：扩展 LLVM SSA op coverage
8. `a36cc472b3`：增加 call op lowering
9. `8b64b9654f`：把部分 runtime/builtin call 降为 LLVM intrinsic

注意：本阶段按项目决策只迁移 `personal/go1.25.master` 的前 6 笔提交，
不使用 `personal/codex/add-llvm-backend-support-for-ssa-ops`。后者的 3 笔
新增 lowering 提交只作为后续能力差异参考，不进入当前构建。

### 2.2 `go-llvm`

路径：`/Volumes/Disk1/00.Work/00.Code/goallc/go-llvm`

- 由 `tinygo.org/x/go-llvm` 派生，module path 为
  `github.com/goallc/go-llvm`；
- LLVM 23 基础兼容及当前清理方案已进入 `master`，HEAD 为
  `33267b8eb4a0`；
- 本项目只支持 GoALLC 定制 LLVM，不再支持系统预安装 LLVM、
  `byollvm`，也不保留 LLVM 14--22 的兼容配置；
- LLVM API 版本和链接模式是两个正交、必选的 build-tag 维度：
  `llvm23` 与 `dynamicllvm`/`staticllvm`；后续 LLVM 24 只新增
  `llvm24` 版本层；
- 路径与版本、链接模式解耦。所有 cgo include/link 路径固定写为
  `${SRCDIR}/llvm/include` 和 `${SRCDIR}/llvm/lib`；
- `llvm` 不入库，由调用方提供软链接。Go 工具链的 `cmd/dist` 根据
  `-llvm-dir` 创建或更新该链接；
- 动态模式链接 payload 中的 `libLLVM`；静态模式只链接 binding
  `${SRCDIR}` 下由 `cmd/dist` 聚合的 `libLLVMGoALLC.a`；
- 没有生成 Go 文件，没有 `LLVMROOT`，也不把路径塞进普通 cgo 的
  `CGO_CPPFLAGS`、`CGO_LDFLAGS` 或全局动态库环境；
- `goallc_ext.go` 已从 Go vendor 私有补丁收回 binding 仓库。
- 自动 CI 暂时只做源码格式检查，不再为每个 PR 从源码冷构建 LLVM；
  项目发布预构建 LLVM payload 后，CI 将直接下载 artifact 并恢复动态、
  静态两种 binding 测试。

Go 仓库依赖 go-llvm `master` 的 pseudo-version：
`v0.0.0-20260727072003-33267b8eb4a0`。

### 2.3 `llvm-project`

路径：`/Volumes/Disk1/00.Work/00.Code/goallc/llvm-project`

Remote：

- `official`: 社区 `llvm/llvm-project`
- `origin`: `zhouguangyuan0718/llvm-project`
- `upstream`: `goallc/llvm-project`

当前分支：

- 正式及 GitHub 默认分支 `llvm23.1.master`
- HEAD `867cfac7abbb`
- 基于本地 `official/release/23.x` 的 `ce6af707aac8`
- LLVM 版本文件为 `23.1.0`
- GoObj 主题分支在 release base 上有 24 笔提交
- 原聚合静态库方案的 GoALLC PR #2 已关闭；其 feature 分支已用
  `e31213b5750d` 撤回全部源码改动，相对 `llvm23.1.master` 的净 diff
  为零。静态聚合职责已移到 Go `cmd/dist`

本轮已恢复 Clang、LLDB 和 MLGO 测试夹具中的既有 symlink/file-type
变化，源码工作区目前干净。旧的 25 GB 构建目录已删除，并从空目录重新完成
CMake configure 和 Ninja 全量构建；随后启用 `LLVM_BUILD_LLVM_DYLIB`
生成供 binding 和 Go compiler 使用的 `libLLVM.23.1git.dylib`。

## 3. 当前真实编译链路

```text
社区 Go 前端
  -> walk / generic Go SSA
  -> 开启 -enablellvm 时调用 LLVMCompile
       -> 每个 package 共用一个 LLVM Module
       -> 输出 <go-compiler-output>.ll
  -> 仍继续运行社区 Go SSA 后端
  -> 正常生成 Go package archive

独立 LLVM GoObj 实验链路
  package 目录中的 .ll
  -> llvm-goobj-toolexec
  -> LLVM optimize + CodeGen
  -> GoObj object
  -> go tool pack 追加到 Go package archive
  -> 社区 Go linker
```

这两条链路的连接点已经具备，但尚未自动连接：

- Go lowering 能输出 `.ll`，但编译器中的 `// TODO Assemble` 尚未实现；
- `llvm-goobj-toolexec` 能把 `.ll` 变为 GoObj，但目前通过扫描 package
  源目录发现 `.ll`，不是直接接收编译器内存中的 module；
- Go compiler 仍负责生成 `__.PKGDEF`、Go stub、原生 `_go_.o` 和 archive；
- LLVM object 作为新增成员追加到 archive。

## 4. 已完成工作

### 4.1 Go 前端与 SSA lowering

personal 主题分支实现了以下原型能力：

- 新增编译器布尔 flag `-enablellvm`；
- LLVM 模式下关闭并行 backend，避免共享的 global LLVM context/module
  并发访问；
- package 初始化时创建 LLVM module；
- 在 Go SSA `Compile` 入口调用 `LLVMCompile`；
- 建立 Go SSA block/value 到 LLVM block/value 的映射；
- 支持 phi 的延迟 incoming edge 填充；
- 支持 `Ret`、`If`、`Plain` 三类 control-flow block；
- 支持整数/浮点常量、布尔、nil、字符串常量；
- 支持主要整数/浮点算术、位运算、比较、移位和数值转换；
- 支持部分 pointer addressing、load/store、struct、string/slice
  len/cap；
- 支持 static、closure、interface 和 tail call 形态的初步 lowering；
- 支持多返回值的 LLVM struct 表示和 `SelectN`；
- 把 `runtime.memmove` 降为 `llvm.memmove`；
- 把 `runtime.memclrNoHeapPointers` 和 `runtime.memclrHasPointers` 降为
  `llvm.memset`；
- 每个函数结束后调用 LLVM verifier；
- 最后把 module 以文本 LLVM IR 写到 `<output>.ll`。

分支还增加了 `cmd/objview`，用于查看 Go archive/GoObj 的 header、block、
symbol、relocation、aux data、function metadata、stack map 和反汇编内容。
它是 GoObj 格式开发时的重要诊断工具。

### 4.2 Go LLVM binding

- 保留了 LLVM C API 的主要 IR、target、bitcode、DIBuilder、pass、
  execution engine 等 Go 封装；
- 只保留当前 LLVM 23 API，并以独立 build tag 为 LLVM 24 迁移预留扩展点；
- 版本选择、链接模式和 payload 路径互不耦合；
- 动态、静态配置完全由代码 build tag 隔离，不生成配置源文件；
- 静态模式使用一个 `libLLVMGoALLC.a`，不在 binding 中计算组件依赖；
- 允许把 binding 直接 vendor 到 `src/cmd/vendor`，供 `cmd/compile`
  内部使用；
- bootstrap build 通过 `compiler_bootstrap` stub 避免第一阶段直接依赖
  LLVM。

### 4.3 LLVM GoObj 对象格式

LLVM 主题分支已经实现：

- `Triple::GoObj` object format；
- GoObj binary format constants、header、block、symbol、relocation 和 aux
  record；
- `MCSectionGoObj`、GoObj streamer、assembler parser 和 object writer；
- X86 和 AArch64 的 GoObj target writer；
- GoObj section 到 Go symbol kind 的映射；
- Go toolchain 需要的 textual object header；
- package path、object flags、symbol ABI 和外部 symbol ref；
- X86/AArch64 relocation 映射；
- compiler/assembly 两种 GoObj source kind；
- `go120ld` object magic，以及与当前 Go 1.25.4 对象布局一致的 record
  sizes 和 enum。

本机安装的 Go 1.26.5 仍使用相同的 `go120ld` magic、record sizes 和
`SSEHUNWINDINFO=25` symbol-kind 尾项。这个观察只能说明核心布局仍匹配，
不能代替升级时对 `cmd/internal/goobj`、`objabi`、ABI 和 linker 读取逻辑的
完整差异审计。

### 4.4 Go ABI 与运行时元数据

LLVM 主题分支已经实现：

- LLVM IR calling conventions `goabiinternal` 和 `goabi0`；
- 参数和返回值的寄存器/栈分类；
- X86_64 和 AArch64 lowering；
- ABI0 stack calling convention；
- register argument spill/home slot 计算；
- 对外部 GoObj reference 记录 ABI；
- stack growth check 和 `runtime.morestack_noctxt` 慢路径；
- `pcsp` stack delta；
- `pcfile` / `pcline`；
- function PC metadata 和基础 FuncInfo；
- empty args/locals stack maps、stack-map index 等必需 aux carrier；
- Go type symbol flag、type descriptor relocation，以及手写 type
  descriptor 的 reflect/interface 端到端验证；
- Darwin arm64 的 GoObj、relocation、calling convention 和 stack growth。

### 4.5 `llvm-goobj-toolexec`

工具目前能够：

- 对非 `compile` 的 Go tool invocation 原样透传；
- 在每次 package compile 时扫描源码目录相邻的 `.ll`；
- 从 LLVM IR 的 `goabi0` / `goabiinternal` 函数收集导出 symbol；
- 生成临时 `symabis`；
- 有 bodyless Go 声明时移除 `-complete`；
- 从 Go compiler 生成的 archive header 推断 GOOS、GOARCH、Go version 和
  shared 标志；
- 设置 GoObj triple、package path 和 codegen options；
- 验证、优化 LLVM IR；
- 生成 GoObj；
- 使用 sibling `pack` 或 `go tool pack` 把 LLVM object 追加到 archive；
- 支持显式 `--llvm-ir`、package filter、target/opt level 等参数。

现有端到端 fixture 覆盖：

- 基础函数、整数/浮点参数、多返回值；
- ABIInternal 和 ABI0；
- 字符串相等；
- runtime stack 可见性；
- GC 调用；
- 大栈帧和 stack growth；
- 手写 Go type descriptor、reflection 和 interface method；
- X86 GoObj，以及 AArch64 Darwin 的目标级验证。

## 5. 尚未完成或存在风险的部分

### 5.1 Go SSA path 仍是 sidecar prototype

- `-enablellvm` 不会替换原生 backend；
- 编译器仍生成原生 Go machine code；
- LLVM module 只写为 `.ll`；
- `compileFunctions` 中的 assemble 阶段仍是 TODO；
- 没有把 LLVM GoObj 自动加入当前 package archive；
- 没有在 Go branch 中增加 lowering regression tests。

### 5.2 Go lowering 尚未采用新的 Go ABI/GoObj 能力

Go SSA 分支形成于 LLVM Go ABI 工作之前，目前：

- 没有给函数设置 `goabiinternal` / `goabi0` calling convention；
- 没有给 call instruction 设置相应 calling convention；
- module 没有 target triple；
- module 没有 target data layout；
- 没有生成 GoObj package-path metadata、symbol flags 或 type metadata；
- 没有把 Go liveness、stack maps、write barriers、defer/panic metadata
  传给 LLVM。

因此 personal Go lowering 输出不能直接被视为正确的 Go ABI LLVM IR。

### 5.3 lowering 正确性缺口

当前源码中已经可以直接识别出以下原型级问题：

- 未支持 SSA op 只打印 `skip value`，不会使编译失败；
- verifier 错误被忽略；
- `OpNilCheck` 没有生成 nil check；
- `OpBitLen32/64` 暂时错误地当作 identity；
- control-flow block 只处理 `Ret`、`If`、`Plain`；
- type lowering 只覆盖有限的 primitive、pointer、function、struct、
  string 和 slice；
- type table 初始化中 `TUINT32` 重复、`TINT32` 缺失，并把一个
  `TUINT8` entry 错写为 `i64`，显然需要在恢复开发前修正；
- global context/module/type cache 的生命周期是 process-global；
- call lowering 只按 LLVM type 拼函数签名，尚未实现 Go ABI、closure
  context、interface dispatch、panic edge 和 tail-call 语义；
- `runtime.memclrHasPointers` 直接替换为 `memset` 需要重新审计 GC/write
  barrier 语义。

### 5.4 GC、调试信息和完整兼容性

- LLVM writer 当前为函数挂接 empty args/locals pointer maps；这不适用于
  一般含 live Go pointers 的函数；
- 没有从 Go liveness 生成精确 stack map；
- type descriptor 目前由 fixture 中手写 LLVM constants 验证，不是由
  Go type system 自动生成；
- Go-compatible DWARF 尚未实现，端到端测试仍使用 `-ldflags=-w`；
- `PKGDEF` 仍由社区 Go compiler 生成，LLVM 侧不生成；
- relocation、symbol kind、aux metadata 和 architecture 只覆盖当前测试
  所需子集；
- 当前重点架构是 X86_64 和 AArch64，远未达到社区 Go 的全平台矩阵。

### 5.5 构建与版本状态

当前可工作的集成组合是：

- Go checkout：官方 `release-branch.go1.27` 的 `go1.27rc2`；
- bootstrap Go：Go 1.26.5；
- vendored/standalone binding：显式选择 `llvm23`；
- LLVM source/build：23.1.0git；
- LLVM payload 默认位于 `$GOROOT/llvm`，也可用 `-llvm-dir` 指定；
- LLVM API 默认版本为 23，也可用 `-llvm-version` 显式指定；
- 默认通过 payload 中的 `libLLVM` 动态链接；`-llvm-link=static` 让
  `cmd/dist` 使用 payload 的
  `llvm-config` 与 `llvm-ar` 生成 binding 本地的 `libLLVMGoALLC.a`。

当前 LLVM 由 macOS 26 SDK 构建，而 Go 1.27 工具链默认链接目标为 macOS
16，因此链接时会产生 deployment-target 警告。构建和 smoke test
成功，但后续应在 CMake 配置中显式统一最低 macOS 版本。

`cmd/dist` 还用一个恒为 true 的 `buildGoallc` 强制所有工具走 external
link。这是早期 bring-up 手段，后续应缩小到真正依赖 LLVM 的 compiler
阶段和受支持平台。`cmd/dist` 现在根据参数自行注入
`llvm23,dynamicllvm` 或 `llvm23,staticllvm`；bootstrap 仍使用
`compiler_bootstrap` stub。LLVM 的 include 和 link 参数只存在于
binding 的代码配置中，不与普通 cgo 构建的外部环境变量混用。

## 6. 当前验证状态

本轮已完成并确认：

- Go 的 6 笔既有提交已迁移到 `official/release-branch.go1.27` 的
  `b7cc93a369`，没有使用更新的
  `add-llvm-backend-support-for-ssa-ops` 分支；
- LLVM 源码工作区清理后，从空 `llvm/cmake-build-debug` 目录配置并完成
  `3025/3025` 个 Ninja 构建目标；
- 新构建产物为 LLVM `23.1.0git`、Debug、assertions enabled，包含 X86
  和 AArch64，并生成约 302 MB 的 `libLLVM.23.1git.dylib`；
- 8 个 X86/AArch64 GoObj MC/CodeGen 定向测试全部通过；
- 旧方案曾由 LLVM 的 `LLVMGoALLC` target 生成聚合库；当前方案已把
  聚合职责移到 `cmd/dist`，LLVM 源码不再需要该项目专用 target；
- standalone `go-llvm` 在不设置外部 `CGO_*` 参数的情况下，以
  `llvm23,dynamicllvm` 和 `llvm23,staticllvm` 完成全测试；
- 更新后的 binding 已同步到 Go vendor，`goallc_ext.go` 不再是 vendor
  私有差异；
- Go 三阶段工具链在默认动态模式和 `-llvm-link=static` 模式下均构建
  成功，`bin/go version` 为
  `go1.27rc2 darwin/arm64`，`compile -h` 包含 `-enablellvm`；
- `go test cmd/dist`、`go test cmd/compile/internal/ssa cmd/objview` 通过；
- 最小 `Add(int64, int64)` 包经 `compile -enablellvm` 同时生成 Go archive
  和 LLVM IR，后者通过 LLVM 23 `opt -passes=verify`。

已有历史验证记录表明，当前 `llvm23.1.master` 的 GoObj 工作在 LLVM 23
rebase 后曾通过：

- X86/AArch64 GoObj MC 与 CodeGen tests；
- `-verify-machineinstrs`；
- `llvm-goobj-toolexec`；
- 原生 `go build` / `go test -toolexec`；
- runtime stack、stack growth、GC trigger 和 type descriptor 测试。

旧 build directory 的失败已通过清理并重新配置解决，不能再把旧 binary 的
结果当作当前源码状态。

## 7. 本轮版本升级结果与剩余门槛

本轮升级保持既有功能范围，不引入
`add-llvm-backend-support-for-ssa-ops` 的额外 lowering。三个版本基线为：

- Go：`official/release-branch.go1.27`，当前 `go1.27rc2`；
- LLVM：`release/23.x` 上的 GoObj 分支 `llvm23.1.master`；
- binding：GoALLC `go-llvm` PR #3，显式 LLVM 23。

### 7.1 已完成的版本迁移

1. LLVM 23 分支和共享库 clean build 已完成。
2. binding 已清理为定制 LLVM 23、固定 `${SRCDIR}/llvm` payload 和正交
   build tags。
3. Go 的 6 笔既有提交已重放到 Go 1.27 release branch。
4. vendored binding、dist build tags 和 Go 1.27 vet 兼容性已更新。
5. 三阶段 bootstrap、核心测试和最小 IR verifier smoke test 已恢复。

### 7.2 后续仍需完成的兼容性门槛

至少应满足：

- 三个仓库 worktree clean，base/head/remote 可追溯；
- 社区 Go toolchain 完整 bootstrap 成功；
- 原生 Go tests 没有因 LLVM hook 产生回归；
- `go-llvm` 对选定 LLVM major 的单元测试通过；
- `-enablellvm` 对最小算术、分支、phi、call、多返回值样例输出可验证 IR；
- LLVM GoObj MC/CodeGen tests 通过；
- X86_64 和 AArch64 的 `-verify-machineinstrs` 通过；
- 使用升级后的社区 Go 执行 `go build` / `go test -toolexec` 通过；
- GoObj format/ABI/linker 差异已经按新 Go release 重新审计；
- 所有测试路径、Go binary 路径和 cache 路径参数化，不保留开发机绝对路径。

## 8. 升级之后的首个集成里程碑

完成 release refresh 后，真正的第一条纵向功能链应限定为：

```text
单 package、无 cgo、无泛型复杂实例、有限 SSA op
  -> 社区 Go 前端和 Go SSA
  -> 带 goabiinternal + target layout 的 LLVM IR
  -> LLVM GoObj
  -> 保留 Go compiler 生成的 PKGDEF
  -> Go package archive
  -> 社区 Go linker
  -> 可运行程序
```

这一里程碑必须同时携带：

- 明确拒绝未支持 op/type/block 的 fail-fast 行为；
- LLVM module/function verifier；
- Go ABI calling convention；
- 最小但正确的 stack map/GC 限制，或明确拒绝含 pointer liveness 的函数；
- 可自动运行的端到端测试。

在这条链稳定前，不宜扩大到完整标准库、全平台、DWARF、cgo 或复杂泛型。

## 9. 当前 LLVM 23 集成构建方法

LLVM 从仓库根目录配置：

```sh
cmake -S llvm -B llvm/cmake-build-debug -G Ninja \
  -DCMAKE_BUILD_TYPE=Debug \
  -DCMAKE_INSTALL_PREFIX=/private/tmp/goallc-llvm23-install \
  -DLLVM_ENABLE_PROJECTS=lld \
  -DLLVM_TARGETS_TO_BUILD='X86;AArch64' \
  -DLLVM_ENABLE_ASSERTIONS=ON \
  -DLLVM_BUILD_TESTS=OFF \
  -DLLVM_INCLUDE_TESTS=ON \
  -DLLVM_BUILD_TOOLS=ON \
  -DBUILD_SHARED_LIBS=OFF \
  -DLLVM_BUILD_LLVM_DYLIB=ON \
  -DLLVM_ENABLE_RTTI=OFF \
  -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
ninja -C llvm/cmake-build-debug -j6
```

准备统一的 LLVM payload 布局：

```sh
$GOROOT/llvm/bin/llvm-config
$GOROOT/llvm/bin/llvm-ar
$GOROOT/llvm/include/llvm-c/Core.h
$GOROOT/llvm/lib/libLLVM.dylib       # dynamic
$GOROOT/llvm/lib/libLLVM*.a          # static component archives
```

不要把 LLVM build 目录全局写入 `DYLD_LIBRARY_PATH`；这会让 Homebrew 的
clang 等 LLVM 工具误加载该开发版共享库。`cmd/compile` 的链接命令已经写入
payload rpath。

验证 standalone binding：

```sh
cd /Volumes/Disk1/00.Work/00.Code/goallc/go-llvm
ln -s /Volumes/Disk1/00.Work/00.Code/goallc/go/llvm llvm
go test -tags='llvm23 dynamicllvm' ./...
# staticllvm 需要先由 Go 的 cmd/dist 在 binding ${SRCDIR} 聚合静态库
go test -tags='llvm23 staticllvm' ./...
```

构建 Go 集成工具链：

```sh
cd /Volumes/Disk1/00.Work/00.Code/goallc/go/src
./make.bash
./make.bash -llvm-dir=/path/to/llvm -llvm-version=23 -llvm-link=dynamic
./make.bash -llvm-dir=/path/to/llvm -llvm-version=23 -llvm-link=static
```

`cmd/dist` 使用显式 `-llvm-dir`，未指定时使用 `$GOROOT/llvm`。LLVM
版本和链接方式同样只接受 `-llvm-version` 与 `-llvm-link` 参数，不读取
项目专用环境变量。它验证 payload 版本和所选链接库，再原子更新
`$GOROOT/src/cmd/vendor/github.com/goallc/go-llvm/llvm` 软链接。真实目录对
binding 不可见，binding 始终只使用 `${SRCDIR}/llvm`。

静态模式下，`cmd/dist` 通过 payload 的 `llvm-config --link-static
--libfiles` 获取标准 LLVM component archives，再通过 `llvm-ar -M`
聚合为
`$GOROOT/src/cmd/vendor/github.com/goallc/go-llvm/libLLVMGoALLC.a`。
它用 LLVM 版本、组件列表以及输入归档的路径、大小和修改时间缓存结果；
LLVM 源码和构建系统本身不包含 GoALLC 专用静态库 target。

最小 IR smoke test：

```sh
/Volumes/Disk1/00.Work/00.Code/goallc/go/pkg/tool/darwin_arm64/compile \
  -enablellvm -p smoke -o /private/tmp/goallc-smoke.o \
  /private/tmp/goallc-smoke.go
/Volumes/Disk1/00.Work/00.Code/goallc/llvm-project/llvm/cmake-build-debug/bin/opt \
  -passes=verify -disable-output /private/tmp/goallc-smoke.o.ll
```
