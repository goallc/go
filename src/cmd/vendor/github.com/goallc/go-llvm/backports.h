
#include "llvm-c/DebugInfo.h"
#include "llvm-c/Types.h"

#ifdef __cplusplus
extern "C" {
#endif

void LLVMGlobalObjectAddMetadata(LLVMValueRef objValue, unsigned KindID, LLVMMetadataRef md);

LLVMMemoryBufferRef LLVMGoWriteThinLTOBitcodeToMemoryBuffer(LLVMModuleRef M);

LLVMMetadataRef LLVMGoGlobalObjectGetMetadata(LLVMValueRef Value,
                                            unsigned Kind);
LLVMMetadataRef LLVMGoMDNodeGetOperand(LLVMMetadataRef Node, unsigned Index);

LLVMMetadataRef LLVMGoDIBuilderCreateFunction(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, const char *LinkageName, size_t LinkageNameLen,
    LLVMMetadataRef File, unsigned Line, LLVMMetadataRef Type,
    LLVMBool LocalToUnit, LLVMBool IsDefinition, unsigned ScopeLine,
    LLVMDIFlags Flags, LLVMBool Optimized, LLVMMetadataRef Annotations);

LLVMMetadataRef LLVMGoDIBuilderCreateAutoVariable(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File, unsigned LineNo, LLVMMetadataRef Type,
    LLVMBool AlwaysPreserve, LLVMDIFlags Flags, uint32_t AlignInBits,
    LLVMMetadataRef Annotations);

LLVMMetadataRef LLVMGoDIBuilderCreateParameterVariable(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, unsigned ArgNo, LLVMMetadataRef File, unsigned LineNo,
    LLVMMetadataRef Type, LLVMBool AlwaysPreserve, LLVMDIFlags Flags,
    LLVMMetadataRef Annotations);

LLVMMetadataRef LLVMGoDIBuilderCreateArrayType(
    LLVMDIBuilderRef Builder, const char *Name, size_t NameLen, uint64_t Size,
    uint32_t AlignInBits, LLVMMetadataRef ElementType,
    LLVMMetadataRef *Subscripts, unsigned NumSubscripts);

LLVMMetadataRef LLVMGoDIBuilderCreateModule(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File);

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
