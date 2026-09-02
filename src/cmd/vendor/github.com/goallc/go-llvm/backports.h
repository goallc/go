
#include "llvm-c/DebugInfo.h"
#include "llvm-c/Types.h"

#ifdef __cplusplus
extern "C" {
#endif

void LLVMGlobalObjectAddMetadata(LLVMValueRef objValue, unsigned KindID, LLVMMetadataRef md);

LLVMMemoryBufferRef LLVMGoWriteThinLTOBitcodeToMemoryBuffer(LLVMModuleRef M);

void LLVMGoDIBuilderInsertDbgValueRecordAtEnd(
    LLVMDIBuilderRef Builder, LLVMValueRef Val, LLVMMetadataRef VarInfo,
    LLVMMetadataRef Expr, LLVMMetadataRef DebugLoc, LLVMBasicBlockRef Block);

void LLVMGoDIBuilderInsertDeclareRecordAtEnd(
    LLVMDIBuilderRef Builder, LLVMValueRef Storage, LLVMMetadataRef VarInfo,
    LLVMMetadataRef Expr, LLVMMetadataRef DebugLoc, LLVMBasicBlockRef Block);

LLVMMetadataRef LLVMGoDIBuilderCreateLabel(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File, unsigned LineNo,
    LLVMBool IsArtificial, LLVMBool AlwaysPreserve);

#ifdef __cplusplus
}
#endif /* defined(__cplusplus) */
