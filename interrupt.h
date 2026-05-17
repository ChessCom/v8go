#ifndef V8GO_INTERRUPT_H
#define V8GO_INTERRUPT_H

#include "isolate.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void IsolateRequestInterruptTerminate(IsolatePtr ptr);
extern void IsolateSetIdle(IsolatePtr ptr, int is_idle);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
