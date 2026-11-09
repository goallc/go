# 参与 GoALLC 开发

[English](CONTRIBUTING.md) | 简体中文

欢迎通过本仓库的 [issue](https://github.com/goallc/go/issues) 和
[PR](https://github.com/goallc/go/pulls) 参与 GoALLC。编译器与运行时兼容性、性能、测试、
文档和构建工具都欢迎贡献。

## 反馈问题或提出建议

先搜索已有 issue，再选择缺陷、性能或功能建议模板。中英文反馈均可。

反馈缺陷时，请提供 GoALLC 版本（`go version`）、操作系统与架构、相关构建参数、
最小复现，以及预期和实际行为。如果已经与官方 Go 对比，也请附上其版本和结果。

反馈性能问题时，请提供基准源码、CPU 型号、工具链版本、构建参数、多轮测量和执行命令。
对比时保持输入和测试设置一致；报告一组用例时，也请保留变慢的结果。

## 准备 PR

从仓库默认分支 `go1.27.master` 开始，每个改动集中解决一个问题。
说明修改后的行为，有关联 issue 时附上链接。

源码构建参见[构建指南](doc/goallc-build.md)，实现位置参见[架构概览](GOALLC.md)。
Go 专用 LLVM pass 位于 [src/cmd/llvmplugin](src/cmd/llvmplugin/README.md)，
LLVM 目标机制位于 [goallc/llvm-project](https://github.com/goallc/llvm-project)。

修改 Go 文件后运行 `gofmt`。行为变更应补充针对性的回归用例，并使用修改后的工具链运行
相关测试。在 PR 中写明测试内容、运行平台和仍待完成的验证；性能改动附上对比方法和数据。
文档改动应检查链接和示例。

修改面向用户的项目信息时，请保持中英文 README 一致。
在 GitHub 上提交 PR，目标分支为 `go1.27.master`。

如果改动依赖尚未合入的 GoALLC LLVM PR，请在 PR 正文中加入未被注释包裹的
`LLVM-PR: goallc/llvm-project#123`，让 CI 选择对应的 LLVM payload。

## 许可证

除另有说明外，Go 源文件使用 BSD 风格的 [LICENSE](LICENSE)。
修改代码时请保留已有版权和许可声明。
