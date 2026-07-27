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
  -> llc -filetype=obj <archive>.ll
       -> LLVM GoObj
  -> cmd/internal/archive 将 GoObj 追加为 _go_.o
  -> cmd/link 读取 __.PKGDEF 和 _go_.o
```

`-llvmironly` 必须和 `-enablellvm` 一起使用。它使 `compile` 在写出 LLVM
IR 和 Go archive 的 `__.PKGDEF` 后结束，不再生成原生 linker object。因而
对被替换的 package 而言，archive 中唯一的 `_go_.o` 必须是 `llc` 生成的
GoObj；`__.PKGDEF` 仍完全由 Go compiler 生成并位于 archive 的第一个成员。

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

`cmd/llvmtoolexec` 只处理 `compile`，其余 Go tool 调用原样透传。它：

1. 为选中的 compile 调用增加 `-enablellvm -llvmironly`；
2. 调用 `llc -filetype=obj`，不传递或重建 GoObj header 配置；
3. 使用 `cmd/internal/archive` 打开 compiler archive，并将对象以 `_go_.o`
   追加进去；
4. 默认删除 IR 和临时 object，`-keep-ir` 可保留 IR 供检查。

wrapper 不解析 `__.PKGDEF`、不反向识别 Go textual header，也不自行写 ar
header。使用 Go 自己的 archive writer 可保证 `__.PKGDEF` 保持第一个 member，
避免 BSD `ar` 插入 `__.SYMDEF` 后破坏 `cmd/link` 的读取约定。

可用 `-package` 或环境变量 `GOALLC_LLVM_PACKAGE` 限定 import path；未设置时
所有 `compile` 调用都会走这条实验链路。`-llc` 或 `GOALLC_LLC` 必须指向带有
GoObj metadata 支持的 GoALLC LLVM 构建。

## 最小使用方式

假定 `$GOROOT` 是本仓库构建出的 Go toolchain，`$LLC` 是对应 LLVM 分支构建
的 `llc`：

```sh
cd "$GOROOT/src/cmd"
go build -o /private/tmp/llvmtoolexec ./llvmtoolexec

cd /path/to/simple-main-package
GOALLC_LLVM_PACKAGE=main \
  "$GOROOT/bin/go" build \
  -toolexec="/private/tmp/llvmtoolexec -llc=$LLC -keep-ir" \
  -o app .
./app
```

在 shell 中也可设置 `GOALLC_LLC="$LLC"`，这样 `-toolexec` 只需指定 wrapper
路径。`-keep-ir` 留下的 IR 路径为 compile 输出 archive 路径加 `.ll` 后缀。

开发时至少运行：

```sh
cd "$GOROOT/src/cmd"
go test ./llvmtoolexec

cd /path/to/llvm-project
llvm/cmake-build-debug/bin/llvm-lit -sv \
  llvm/test/CodeGen/AArch64/goobj-ir-config.ll
```

此外应以一个简单 `main` package 执行真实的 `go build -toolexec`，并运行产物。
这同时验证 compiler archive、IR metadata、`llc`、GoObj archive member 和
`cmd/link` 的接口。

## 当前范围与后续工作

- 当前 target triple 仅配置了 `darwin/arm64` 和 `linux/amd64`。
- LLVM SSA lowering 仍不完整；复杂 SSA op、GC pointer liveness、defer/panic、
  closure/interface、完整 ABI/DWARF 等尚未达到通用正确性。
- 每个经 LLVM 替换的 package 都必须由 `llc` 生成完整的 `_go_.o`，不能与
  原生 compiler object 混合。
- `!goobj.config` 是这条开发链路的稳定交接点。新增 header 配置时应新增独立
  metadata 字段和相应 `llc` 验证，不要恢复 wrapper 中的 header 解析逻辑。

相关合入记录：Go [#3](https://github.com/goallc/go/pull/3)，LLVM
[#3](https://github.com/goallc/llvm-project/pull/3)。
