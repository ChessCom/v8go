#ifndef V8GO_HEAP_LIMIT_H
#define V8GO_HEAP_LIMIT_H

#include "isolate.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void IsolateAddCustomNearHeapLimitCallback(IsolatePtr ptr);
extern void IsolateRemoveCustomNearHeapLimitCallback(IsolatePtr ptr,
                                                     size_t heap_limit);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
