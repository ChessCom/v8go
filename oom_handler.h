#ifndef V8GO_OOM_HANDLER_H
#define V8GO_OOM_HANDLER_H

#include "isolate.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void IsolateSetOOMErrorHandler(IsolatePtr ptr);
extern void IsolateClearOOMErrorHandler(IsolatePtr ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
