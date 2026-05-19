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

// Zero-copy: wraps external (Go-owned) memory as an ArrayBuffer.
// The deleter_ref is passed back to Go when V8 releases the backing store.
extern ValuePtr NewArrayBufferExternal(ContextPtr ctx_ptr,
                                       void* data,
                                       size_t byte_length,
                                       int deleter_ref);

extern void* ArrayBufferGetData(ValuePtr ptr);
extern size_t ArrayBufferGetByteLength(ValuePtr ptr);
extern BackingStorePtr ArrayBufferGetBackingStore(ValuePtr ptr);

extern int V8SandboxIsEnabled();

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
