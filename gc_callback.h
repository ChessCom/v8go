#ifndef V8GO_GC_CALLBACK_H
#define V8GO_GC_CALLBACK_H

#include "isolate.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void IsolateAddGCPrologueCallback(IsolatePtr ptr);
extern void IsolateRemoveGCPrologueCallback(IsolatePtr ptr);
extern void IsolateAddGCEpilogueCallback(IsolatePtr ptr);
extern void IsolateRemoveGCEpilogueCallback(IsolatePtr ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
