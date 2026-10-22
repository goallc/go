// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Go assembler's structured bridge to LLVM's decoder and instruction printer.
#ifndef LLVM_GO_MC_BINDINGS_H
#define LLVM_GO_MC_BINDINGS_H
#include <stddef.h>
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
typedef struct {
  uint64_t Offset;
  unsigned Size;
  unsigned PCRelative;
  int64_t Addend;
  const char *Symbol;
} LLVMGoMCReloc;
void *LLVMGoCreateMCDecoder(const char *Triple, const char *Features);
void LLVMGoDisposeMCDecoder(void *Decoder);
uint64_t LLVMGoMCInstructionSize(void *Decoder, const uint8_t *Bytes,
                                 size_t Length, uint64_t PC);
// Strings returned by this interface are owned by the caller (free).
char *LLVMGoDecodeMCInstruction(void *Decoder, const uint8_t *Bytes,
                                size_t Length, uint64_t PC,
                                const LLVMGoMCReloc *Relocs, size_t NumRelocs,
                                uint64_t *Size, int64_t *Target, char **Error);
#ifdef __cplusplus
}
#endif
#endif
