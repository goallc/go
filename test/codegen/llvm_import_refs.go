// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	_ "encoding/hex"
	"fmt"
	"os"
	"reflect"
)

// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE autolib package="encoding/hex" fingerprint=[[HEX:[0-9a-f]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE autolib package="fmt" fingerprint=[[FMT:[0-9a-f]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE autolib package="os" fingerprint=[[OS:[0-9a-f]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE autolib package="reflect" fingerprint=[[REFLECT:[0-9a-f]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE package index=0 path=""
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE reference name="reflect.(*rtype).Elem" class=nonpackage_reference
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE reference name="reflect.(*rtype).Kind" class=nonpackage_reference
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE relocation {{.*}} type=R_CALL {{.*}} target_kind=imported target_package="fmt" target_name="fmt.Sprintf" target_index=[[FMT_INDEX:[0-9]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE relocation-count type=R_USEIFACE count=[[USEIFACE:[0-9]+]]
// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE relocation-count type=R_USENAMEDMETHOD count=[[USENAMEDMETHOD:[0-9]+]]
// LLVM-NATIVE-OBJSUMMARY: LLVM autolib package="encoding/hex" fingerprint=[[HEX]]
// LLVM-NATIVE-OBJSUMMARY: LLVM autolib package="fmt" fingerprint=[[FMT]]
// LLVM-NATIVE-OBJSUMMARY: LLVM autolib package="os" fingerprint=[[OS]]
// LLVM-NATIVE-OBJSUMMARY: LLVM autolib package="reflect" fingerprint=[[REFLECT]]
// LLVM-NATIVE-OBJSUMMARY: LLVM package index=0 path=""
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM reference name="reflect.(*rtype).Elem" class=nonpackage_reference
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM reference name="reflect.(*rtype).Kind" class=nonpackage_reference
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM relocation {{.*}} type=R_CALL {{.*}} target_kind=imported target_package="fmt" target_name="fmt.Sprintf" target_index=[[FMT_INDEX]]
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM relocation-count type=R_USEIFACE count=[[USEIFACE]]
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM relocation-count type=R_USENAMEDMETHOD count=[[USENAMEDMETHOD]]
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM symbol name={{"codegen.llvmImportBox.*"}} kind=STEXT flags={{.*}}dupok
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM symbol name={{".goallc.anon.*"}} kind={{.*}} flags={{.*}}local

//go:noinline
func llvmImportBox[T any](value T) any {
	return value
}

func llvmImportedReferences() string {
	boxed := llvmImportBox(7)
	if reflect.TypeOf(boxed).Name() != "int" {
		return ""
	}
	if reflect.TypeOf((*int)(nil)).Elem().Kind() != reflect.Int {
		return ""
	}
	return fmt.Sprintf("%s/%v", os.Getenv("GOALLC_IMPORT_VALUE"), boxed)
}
