#ifndef V8GO_EXTERNAL_STRING_H
#define V8GO_EXTERNAL_STRING_H

#include "isolate.h"

#ifdef __cplusplus
#include "context.h"
extern "C" {
#else
typedef struct m_ctx m_ctx;
typedef m_ctx* ContextPtr;
typedef struct m_value m_value;
typedef m_value* ValuePtr;
#endif

extern ValuePtr NewExternalOneByteString(ContextPtr ctx_ptr,
                                         const char* data,
                                         size_t length);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
