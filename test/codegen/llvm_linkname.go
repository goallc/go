// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import _ "unsafe"

//go:linkname llvmLinknameExternal runtime.llvmLinknameExternal
func llvmLinknameExternal() int

//go:linkname llvmLinknameLocal runtime.llvmLinknameLocal
func llvmLinknameLocal() int {
	return 7
}

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmLinknameCalls()
// LLVM: call goabiinternal i64 @"runtime.llvmLinknameExternal<linkname>"()
// LLVM: declare goabiinternal i64 @"runtime.llvmLinknameExternal<linkname>"()
// LLVM-NOT: @"runtime.llvmLinknameLocal<linkname>"
// LLVM-LABEL: define weak goabi0 void @"runtime.llvmLinknameLocal<ABI0>"(
// LLVM-SAME: ptr goret(i64) align 8 "goretindex"="0" [[RESULT_HOME:%[^)]+]])
// LLVM: [[RESULT:%.*]] = call goabiinternal i64 @runtime.llvmLinknameLocal()
// LLVM-NEXT: store i64 [[RESULT]], ptr [[RESULT_HOME]], align 8
// LLVM-LABEL: define goabiinternal i64 @runtime.llvmLinknameLocal()
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmLinknameCalls()
// LLVM-OPT: call goabiinternal i64 @"runtime.llvmLinknameExternal<linkname>"()
// LLVM-OPT: declare goabiinternal i64 @"runtime.llvmLinknameExternal<linkname>"()
// LLVM-OPT-NOT: @"runtime.llvmLinknameLocal<linkname>"
// LLVM-OPT-LABEL: define weak goabi0 void @"runtime.llvmLinknameLocal<ABI0>"(
// LLVM-OPT-SAME: ptr {{.*}}goret(i64) align 8{{.*}} "goretindex"="0" [[OPT_RESULT_HOME:%[^)]+]])
// LLVM-OPT: store i64 7, ptr [[OPT_RESULT_HOME]], align 8
// LLVM-OPT-LABEL: define goabiinternal {{.*}}@runtime.llvmLinknameLocal()
func llvmLinknameCalls() int {
	return llvmLinknameExternal() + llvmLinknameLocal()
}
