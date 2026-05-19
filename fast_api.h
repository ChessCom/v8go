#ifndef V8GO_FAST_API_H
#define V8GO_FAST_API_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Mirrors v8::CTypeInfo::Type
typedef enum {
  kCTypeVoid = 0,
  kCTypeBool = 1,
  kCTypeUint8 = 2,
  kCTypeInt32 = 3,
  kCTypeUint32 = 4,
  kCTypeInt64 = 5,
  kCTypeUint64 = 6,
  kCTypeFloat32 = 7,
  kCTypeFloat64 = 8,
  kCTypePointer = 9,
  kCTypeV8Value = 10,
  kCTypeSeqOneByteString = 11,
} CTypeInfoType;

// Opaque handle to a heap-allocated CFunctionInfo + CTypeInfo array.
typedef void* CFunctionInfoPtr;

// Build a CFunctionInfo from raw type descriptors.
// arg_types is an array of arg_count CTypeInfoType values.
// The first arg is always the receiver (v8::Local<v8::Object>).
extern CFunctionInfoPtr BuildCFunctionInfo(CTypeInfoType return_type,
                                           const CTypeInfoType* arg_types,
                                           int arg_count);

// Free a CFunctionInfo built with BuildCFunctionInfo.
extern void FreeCFunctionInfo(CFunctionInfoPtr ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
