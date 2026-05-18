#ifndef V8GO_MODULE_H
#define V8GO_MODULE_H

#include "errors.h"

#ifdef __cplusplus

#include "deps/include/v8-script.h"

namespace v8 {
class Isolate;
class Module;
}  // namespace v8

struct m_module {
  v8::Persistent<v8::Module> ptr;
  v8::Isolate* iso;
};

typedef v8::Isolate v8Isolate;

extern "C" {
#else

typedef struct v8Isolate v8Isolate;

#endif

typedef v8Isolate* IsolatePtr;
typedef struct m_ctx m_ctx;
typedef m_ctx* ContextPtr;
typedef struct m_value m_value;
typedef m_value* ValuePtr;

typedef struct m_module m_module;
typedef m_module* ModulePtr;

typedef struct {
  ModulePtr ptr;
  RtnError error;
} RtnModule;

extern RtnModule CompileESModule(ContextPtr ctx_ptr,
                                 const char* source,
                                 const char* origin);

extern int ModuleGetStatus(IsolatePtr iso_ptr, ModulePtr mod_ptr);
extern int ModuleGetRequestsLength(IsolatePtr iso_ptr, ModulePtr mod_ptr);
extern const char* ModuleGetRequest(ContextPtr ctx_ptr,
                                    ModulePtr mod_ptr,
                                    int index);
extern int ModuleGetIdentityHash(IsolatePtr iso_ptr, ModulePtr mod_ptr);

extern int ModuleInstantiate(ContextPtr ctx_ptr, ModulePtr mod_ptr);
extern RtnValue ModuleEvaluate(ContextPtr ctx_ptr, ModulePtr mod_ptr);
extern ValuePtr ModuleGetNamespace(ContextPtr ctx_ptr, ModulePtr mod_ptr);
extern void ModuleFree(ModulePtr mod_ptr);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
