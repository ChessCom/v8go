#ifndef V8GO_ARRAYBUFFER_H
#define V8GO_ARRAYBUFFER_H

#include "v8go.h"

#ifdef __cplusplus
#include "context.h"
extern "C" {
#else
typedef struct m_ctx m_ctx;
typedef m_ctx* ContextPtr;
typedef struct m_value m_value;
typedef m_value* ValuePtr;
#endif

extern ValuePtr NewArrayBufferFromBytes(ContextPtr ctx_ptr,
                                        const void* data,
                                        size_t byte_length);

extern ValuePtr NewArrayBufferAlloc(ContextPtr ctx_ptr,
                                    size_t byte_length);

extern void* ArrayBufferGetData(ValuePtr ptr);
extern size_t ArrayBufferGetByteLength(ValuePtr ptr);
extern BackingStorePtr ArrayBufferGetBackingStore(ValuePtr ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
