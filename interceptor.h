#ifndef V8GO_INTERCEPTOR_H
#define V8GO_INTERCEPTOR_H

#include "template.h"

#ifdef __cplusplus
extern "C" {
#endif

extern void ObjectTemplateSetNamedPropertyHandler(
    TemplatePtr ptr,
    int callback_ref,
    int has_getter,
    int has_setter,
    int has_query,
    int has_deleter,
    int has_enumerator);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
