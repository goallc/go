# GoALLC 构建、安装与缓存约定

本文固化 GoALLC LLVM payload、pass plugin 和 Go toolchain 的构建边界。Go
toolchain 的构建入口仍是标准的 `src/make.bash`；LLVM 仓库中的辅助脚本只负责
准备完整的 LLVM payload，不建立第二套 Go 构建入口。

## 产物边界

构建包含三个有顺序的阶段：

1. `llvm-project` 生成 LLVM build tree；
2. `cmake --install` 生成独立、完整的 LLVM payload；
3. Go `make.bash` 使用该 payload 构建匹配的 pass plugin 和 Go toolchain。

LLVM build tree 是 Ninja 的增量工作区，不是可供 Go 使用的 payload。有效 payload
必须是一个标准安装树，至少包含：

```text
bin/llvm-config
bin/llvm-ar
bin/llc
bin/opt
bin/FileCheck
include/llvm-c/Core.h
include/llvm/Config/llvm-config.h
lib/cmake/llvm/LLVMConfig.cmake
lib/cmake/llvm/AddLLVM.cmake
lib/libLLVM.{dylib,so}
lib/libLLVMCore.a
```

源码头文件、build tree 中的生成头文件、工具、CMake package、共享库和静态组件库
必须来自同一次安装。以下混合目录不是合法 payload：

```text
llvm/include  # 从源码/build tree 手工复制
llvm/bin      # 指向 cmake-build/bin 的软链接
llvm/lib      # 指向 cmake-build/lib 的软链接
```

`cmd/dist` 会核对 `llvm-config --prefix`、生成头文件和 CMake 开发文件，遇到上述
混合布局时直接失败。

## 规范构建流程

首先使用 llvm-project 仓库拥有的构建器准备 LLVM payload：

```sh
cd /path/to/llvm-project
./llvm/utils/goallc/build-payload.bash
```

脚本默认使用自身所在的 llvm-project checkout，配置 Release + assertions，构建
X86 和 AArch64，并安装到 llvm-project 根下独立的
`build-goallc-payload-release`。它不构建 plugin 或 Go。

然后通过 Go 原有入口构建 toolchain；`make.bash` 内部的 `cmd/dist` 会校验
payload、构建并原子安装匹配的 plugin，再完成正常的三阶段构建：

```sh
cd src
GOALLC_CCACHE=/path/to/ccache \
./make.bash \
  -llvm-dir=/path/to/llvm-payload \
  -llvm-version=23 \
  -llvm-link=dynamic
```

LLVM payload 脚本的常用配置通过环境变量指定：

```sh
GOALLC_LLVM_SOURCE=/path/to/llvm-project \
GOALLC_LLVM_BUILD=/path/to/llvm-build \
GOALLC_LLVM_INSTALL=/path/to/llvm-payload \
GOALLC_BUILD_TYPE=Release \
GOALLC_LLVM_TARGETS='X86;AArch64' \
GOALLC_MACOS_DEPLOYMENT_TARGET=13.0 \
GOALLC_BUILD_JOBS=12 \
/path/to/llvm-project/llvm/utils/goallc/build-payload.bash
```

将同一个 `GOALLC_LLVM_INSTALL` 路径显式传给 `make.bash -llvm-dir`。不要再把
`$GOROOT/llvm` 手工维护成混合目录；如需保留兼容入口，可让整个 `$GOROOT/llvm`
软链接到这个安装树。

payload 还必须位于 LLVM build tree 之外。LLVM 的 `llvm-config` 会把 build tree
内部的路径识别为 development layout，即使那里是 `cmake --install` 的结果，
其 prefix 和库路径仍会回到 build tree。

## ccache

LLVM payload 脚本优先使用显式的 `GOALLC_CCACHE`，然后检测 Apple Silicon
Homebrew 的 `/opt/homebrew/bin/ccache`，最后查找 `PATH` 中的 `ccache`。
构建 Go 时把同一个 `GOALLC_CCACHE` 传给 `make.bash`，`cmd/dist` 会将其配置为
plugin 的 C/C++ compiler launcher。

本机推荐配置为：

```sh
export GOALLC_CCACHE=/opt/homebrew/bin/ccache
export CCACHE_DIR=/Volumes/Disk1/00.Work/.cache/ccache
ccache -M 100G
```

可用以下命令确认已有 CMake tree 没有绕过缓存：

```sh
grep 'CMAKE_.*_COMPILER_LAUNCHER' "$GOALLC_LLVM_BUILD/CMakeCache.txt"
ninja -C "$GOALLC_LLVM_BUILD" -t commands | grep ccache | head
ccache -s
```

ccache 只负责加速，不参与正确性判断。

Darwin 默认额外设置 `CMAKE_OSX_DEPLOYMENT_TARGET=13.0`，与当前 Go linker 在
`cmd/link/internal/ld/macho.go` 中写入的最低支持版本一致。否则 CMake 会继承
当前 SDK（例如 26.0），动态链接产生版本警告，static aggregate 中的每个 LLVM
object 都会携带过高的最低系统版本。Go 更新最低支持版本时应同步这里；特殊构建
可用 `GOALLC_MACOS_DEPLOYMENT_TARGET` 覆盖。`cmd/dist` 会从 payload manifest
读取该值并用于 plugin CMake；非规范脚本生成的 payload 没有 manifest 时，可在
运行 `make.bash` 时显式设置标准的 `MACOSX_DEPLOYMENT_TARGET`。

## 安装的原子性

脚本先用 `DESTDIR` 把 LLVM 安装到 payload 同一文件系统中的临时目录，并在临时
目录检查工具、生成头文件、CMake package、动态库和静态组件库。随后整体切换
目录，再在正式路径执行 `llvm-config` 检查 prefix、版本和 target；旧 payload
保留到这些检查全部通过，失败时自动恢复。LLVM 构建或安装中断不会把半套新文件
留在正式 payload 中。

安装后生成：

```text
$LLVM_PAYLOAD/share/goallc/build-manifest
```

它记录 LLVM revision、dirty 状态、LLVM 版本、build type、targets、build 目录、
安装前缀、ccache 路径和 static system-library closure，供诊断使用；时间戳和
manifest 本身不作为编译缓存身份。

## 发布的 LLVM payload 与 CI

GoALLC LLVM payload 发布在
[`goallc/llvm-project` Releases](https://github.com/goallc/llvm-project/releases)。
当前 Linux amd64 和 arm64 CI 固定使用：

```text
tag:          goallc-llvm23.1.0-20260812T002702Z
revision:     d9a0149bdda5b9fb1f63bf5c8cd2dbe71d7bf523
amd64 asset: goallc-llvm23.1.0-20260812T002702Z-linux-amd64.tar.zst
arm64 asset: goallc-llvm23.1.0-20260812T002702Z-linux-arm64.tar.zst
```

发布 tag 由 llvm-project 的 `.github/workflows/goallc-release.yml` 构建；归档和
对应 `.sha256` 文件同时上传。Go CI 在 `.github/workflows/goallc.yml` 中固定
release tag 和 LLVM revision，由 release tag 推导 asset 名，并下载同版本的
checksum 文件校验 digest、relocatable prefix、manifest、工具、头文件、CMake
package 和动态依赖，再运行 `make.bash`。禁止依赖 `latest` release、系统 LLVM
或可变 URL。

升级 LLVM 时按以下顺序操作：先从新的 llvm-project commit 发布新 tag，确认
Release asset 可重定位且 checksum 稳定，再在一个 Go PR 中同时更新固定的 release
tag 和 revision；asset 前缀由 release tag 推导。旧 release 和 checksum 不覆盖，
以便历史 Go commit 的 CI 可以复现。

需要同时修改 LLVM 和 Go 时，先为 LLVM 改动建立一个目标为
`llvm23.1.master` 的 PR。LLVM 的 `GoALLC LLVM CI` 会从该 PR 的 head revision
原生构建、测试并打包 Linux amd64 和 arm64 payload；两个任务全部成功后，受信的
发布工作流只校验并转存归档和 checksum，不会执行 PR 产物。产物暂存在滚动的
`goallc-llvm-ci` prerelease 中，LLVM PR 更新时替换为新的 revision，关闭时删除。

在依赖它的 Go PR 描述中加入唯一一行：

```text
LLVM-PR: goallc/llvm-project#123
```

Go CI 会解析 LLVM PR 的当前 head revision，等待两个架构的 payload 发布，按
release checksum 校验归档，并要求 payload manifest 中的 `llvm_revision` 与该
head 完全相同。没有 `LLVM-PR` 行时仍使用工作流内固定的正式 release。修改 Go PR
描述会重新运行 CI；LLVM PR 再次 push 后，需要重跑 Go PR CI 才会选择新的 head。
联合构建产物只用于 PR 验证，合入 Go 前仍须先发布不可变的时间戳 LLVM release，
再把 Go 工作流固定的 release 和 revision 更新到该正式版本。

## LLVM 测试工具选择

`cmd/internal/testdir` 的 LLVM 测试在开始运行测试策略前只选择一次 payload，顺序为
显式的 `GOALLC_LLVM_DIR`、`make.bash` 写入的
`$GOROOT/pkg/goallc-llvm-payload`，最后才是兼容入口 `$GOROOT/llvm`。测试框架只把
该根目录交给 compiler，不再解析或注入 `llc`、`opt`、pass plugin 或 toolexec。

测试启动日志只打印 Go、payload 和进程内优化 pipeline。白名单、灰名单和标准库
runtime 用例只通过 `-gcflags=all=-enablellvm` 选择 LLVM；codegen 用例也运行完整
进程内 pipeline，并以 `-llvm-keep-ir` 读取 compiler 留下的优化前/后 IR。
`FileCheck` 是唯一额外测试工具，其选择仍被约束在同一 payload；外部
LLVM 工具只用于 LLVM 项目自身的格式级测试，不参与 Go 测试执行。

## plugin 的构建与缓存

不需要在 `make.bash` 前手工构建或复制 plugin。`cmd/dist` 使用 payload 自己的
`LLVMConfig.cmake` 配置 `src/cmd/llvmplugin`，生成后以新 inode 原子安装到 Go
toolchain 自己的目录：

```text
$GOROOT/pkg/goallc-llvmplugin/lib/GoALLCStatepoints.dylib # Darwin
$GOROOT/pkg/goallc-llvmplugin/lib/GoALLCStatepoints.so    # Linux
$GOROOT/pkg/goallc-llvmplugin/lib/libGoALLCStatepointsStatic.a
```

LLVM payload 只提供 LLVM headers、CMake package、库和工具；构建过程不向其中
安装 plugin，compiler 运行时也不会去 payload 的 `lib`/`lib64` 中查找 plugin。

plugin 输入身份包含：

- Go 仓库中的 plugin C/C++/CMake 源码；
- payload 中安装的 LLVM/LLVM-C headers 和 LLVM CMake package；
- `llc`、`llvm-config` 和动态 `libLLVM` 的内容。

身份变化时，`cmd/dist` 删除旧的 plugin CMake/Ninja 工作区后重新配置；编译仍走
ccache。这样即使 payload 来自保留旧 mtime 的压缩包，也不会因为 Ninja 的时间戳
判断复用旧 object。安装 plugin 的内容哈希记录在：

```text
$GOROOT/pkg/goallc-llvmplugin/goallc-plugin.stamp
```

构建完成后可独立运行 plugin 和 Go 基础设施验证：

```sh
LLVM_PAYLOAD=/path/to/llvm-payload
PLUGIN_BUILD="$GOROOT/pkg/goallc-llvmplugin"
PLUGIN="$PLUGIN_BUILD/lib/GoALLCStatepoints.dylib" # Linux 使用 .so

"$GOROOT/bin/go" build -o "$PLUGIN_BUILD/goallc-objview" cmd/objview
cmake -S "$GOROOT/src/cmd/llvmplugin" -B "$PLUGIN_BUILD" -G Ninja \
  -DLLVM_DIR="$LLVM_PAYLOAD/lib/cmake/llvm" \
  -DCMAKE_INSTALL_PREFIX="$PLUGIN_BUILD" \
  -DBUILD_TESTING=ON \
  -DGOALLC_OBJVIEW_EXECUTABLE="$PLUGIN_BUILD/goallc-objview"
cmake --build "$PLUGIN_BUILD" --target GoALLCStatepoints
ctest --test-dir "$PLUGIN_BUILD" --output-on-failure
"$GOROOT/bin/go" test cmd/dist cmd/internal/llvmbackend cmd/go/internal/work
"$GOROOT/bin/go" test cmd/internal/testdir -run '^TestLLVMTestPolicy$'
"$GOROOT/bin/go" test cmd/internal/testdir -run '^TestLLVM/codegen/'
```

完整语言特性矩阵仍由 LLVM 白/灰/黑名单测试负责。白名单失败会让 CI 失败；
灰名单总是运行并报告结果，但失败不影响 CI；黑名单完全不运行，并且只允许记录
已知会超时或耗尽内存的用例。

## Go action cache

LLVM 模式下，`cmd/go` 使用带 `-enablellvm` 的 `compile -V=full` probe。
compiler 把自身 build ID、Go toolchain 目录中与 payload ABI 同步构建的
plugin artifact，以及 payload 中存在的动态 `libLLVM` 内容合入 tool identity；
optimization pipeline 等 compiler flags 仍由普通 action input 标识。因此在
相同路径替换 LLVM 或 plugin 会使 LLVM package 失效重编，没有启用 LLVM 的
package 保持原生 Go cache key。静态模式的 plugin 和 LLVM archive 本身还会直接
参与 compiler 链接。

仓库根目录不提交 `VERSION`。开发工具链由 Git revision 生成带 `devel` 的版本，
普通 compiler action 使用真实 binary build ID；LLVM action 在此基础上再合入
上述运行时 artifacts。固定 `VERSION` 会让 fork 被误识别为 release compiler，
从而可能只用不变的版本字符串判断工具身份。

不要用 `go clean -cache` 掩盖身份错误。只有在排查 action-cache 实现本身时才做
clean-cache A/B 对比。

## static LLVM

静态 binding 仍通过标准 Go 构建入口选择：

```sh
cd "$GOROOT/src"
GOALLC_CCACHE=/path/to/ccache \
./make.bash \
  -llvm-dir=/path/to/llvm-payload \
  -llvm-version=23 \
  -llvm-link=static
```

标准 LLVM install 会包含 LLVM 组件 archive；`cmd/dist` 根据
`llvm-config --link-static --libfiles` 聚合它们。`-lm`、`-lz`、`-lzstd`、
`-lxml2` 等平台 system libraries 不复制进 aggregate，而由 go-llvm 的平台
LDFLAGS 在最终链接时解析，因此 Linux 仍需安装相应 development package。
GoALLC plugin 同时构建为
`$GOROOT/pkg/goallc-llvmplugin/lib/libGoALLCStatepointsStatic.a` 并直接链接进
`cmd/compile`；静态模式不会再加载 plugin DSO，也不从 LLVM payload 取 plugin。
`llvm-config` 返回未知 system library，或最终链接找不到依赖时必须失败，不能退回
系统 LLVM。

Darwin 的 zstd 通过 `pkg-config libzstd` 取得实际 Homebrew/MacPorts library
目录；只追加裸 `-lzstd` 在库不位于系统默认搜索路径时仍会链接失败。
`CMAKE_OSX_DEPLOYMENT_TARGET` 只能约束本次构建的 LLVM/plugin object；如果
包管理器提供的 zstd 本身要求更高版本的 macOS，链接器会明确告警，最终工具链的
实际运行下限也会被该外部 dylib 抬高。需要发布到较旧系统时，必须改用以相同
deployment target 构建的 zstd，不能把告警当作 payload 缓存问题处理。

## 手工构建时的最低要求

需要调试脚本本身时，可以手工执行 CMake，但仍必须遵守完整安装边界：

```sh
cmake -S /path/to/llvm-project/llvm -B /path/to/llvm-build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX=/path/to/llvm-payload \
  -DCMAKE_INSTALL_MESSAGE=NEVER \
  -DCMAKE_C_COMPILER_LAUNCHER=/path/to/ccache \
  -DCMAKE_CXX_COMPILER_LAUNCHER=/path/to/ccache \
  -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0 \
  -DLLVM_TARGETS_TO_BUILD='X86;AArch64' \
  -DLLVM_ENABLE_ASSERTIONS=ON \
  -DLLVM_BUILD_TOOLS=ON \
  -DLLVM_BUILD_UTILS=ON \
  -DLLVM_INSTALL_UTILS=ON \
  -DLLVM_INCLUDE_TESTS=ON \
  -DLLVM_BUILD_LLVM_DYLIB=ON \
  -DLLVM_LINK_LLVM_DYLIB=ON
cmake --build /path/to/llvm-build --parallel 12
cmake --install /path/to/llvm-build

cd "$GOROOT/src"
MACOSX_DEPLOYMENT_TARGET=13.0 \
  ./make.bash -llvm-dir=/path/to/llvm-payload -llvm-version=23 -llvm-link=dynamic
```

不要设置全局 `DYLD_LIBRARY_PATH`、`LD_LIBRARY_PATH`、`CGO_CPPFLAGS` 或
`CGO_LDFLAGS` 来补齐路径。Go binding 使用选定 payload 的固定 include/lib 和
rpath；需要这些全局变量才能成功通常说明 payload 不完整或选错了 LLVM。

LLVM build/install、plugin build、Go `make.bash` 和验证测试必须按顺序执行，
不要在同一个 `$GOROOT/pkg` 或 payload 上并发运行。
