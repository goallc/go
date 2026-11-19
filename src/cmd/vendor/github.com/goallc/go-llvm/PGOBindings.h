#ifndef GO_LLVM_PGO_BINDINGS_H
#define GO_LLVM_PGO_BINDINGS_H

#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef LLVMErrorRef (*LLVMGoInstrProfLowering)(LLVMModuleRef);
LLVMErrorRef LLVMGoRunPassesWithPGO(LLVMModuleRef Module,
                                    LLVMTargetMachineRef Machine,
                                    const char *Pipeline, const char *Profile,
                                    LLVMGoInstrProfLowering Lower);
#ifdef __cplusplus
}
#endif
#endif
