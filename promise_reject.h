#ifndef V8GO_PROMISE_REJECT_H
#define V8GO_PROMISE_REJECT_H

#include "isolate.h"
#include "value.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void IsolateSetPromiseRejectCallback(IsolatePtr ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
