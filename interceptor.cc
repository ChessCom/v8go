#include "_cgo_export.h"

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-template.h"

#include "context.h"
#include "interceptor.h"
#include "template-macros.h"
#include "value.h"

using namespace v8;

static Intercepted namedPropertyGetterTrampoline(
    Local<Name> property,
    const PropertyCallbackInfo<Value>& info) {
  Isolate* iso = info.GetIsolate();
  Local<Context> local_ctx = iso->GetCurrentContext();
  int ctx_ref = local_ctx->GetEmbedderData(1).As<Integer>()->Value();
  int callback_ref = info.Data().As<Integer>()->Value();

  String::Utf8Value prop(iso, property);

  m_value* result = goNamedPropertyGetterCallback(
      reinterpret_cast<uintptr_t>(iso), ctx_ref, callback_ref, *prop);

  if (result == nullptr) {
    return Intercepted::kNo;
  }
  info.GetReturnValue().Set(result->ptr.Get(iso));
  return Intercepted::kYes;
}

static Intercepted namedPropertySetterTrampoline(
    Local<Name> property,
    Local<Value> value,
    const PropertyCallbackInfo<void>& info) {
  Isolate* iso = info.GetIsolate();
  Local<Context> local_ctx = iso->GetCurrentContext();
  int ctx_ref = local_ctx->GetEmbedderData(1).As<Integer>()->Value();
  m_ctx* ctx_ptr = goContext(ctx_ref);
  int callback_ref = info.Data().As<Integer>()->Value();

  String::Utf8Value prop(iso, property);

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, Global<Value>(iso, value));
  ValuePtr tracked_val = tracked_value(ctx_ptr, val);

  int intercepted = goNamedPropertySetterCallback(
      reinterpret_cast<uintptr_t>(iso), ctx_ref, callback_ref, *prop, tracked_val);

  return intercepted ? Intercepted::kYes : Intercepted::kNo;
}

extern "C" {

void ObjectTemplateSetNamedPropertyHandler(
    TemplatePtr ptr,
    int callback_ref,
    int has_getter,
    int has_setter,
    int has_query,
    int has_deleter,
    int has_enumerator) {
  LOCAL_TEMPLATE(ptr);

  Local<Integer> cbData = Integer::New(iso, callback_ref);

  NamedPropertyGetterCallback getter = has_getter ? namedPropertyGetterTrampoline : nullptr;
  NamedPropertySetterCallback setter = has_setter ? namedPropertySetterTrampoline : nullptr;

  Local<ObjectTemplate> obj_tmpl = tmpl.As<ObjectTemplate>();
  NamedPropertyHandlerConfiguration config(
      getter,
      setter,
      nullptr,  // query
      nullptr,  // deleter
      nullptr,  // enumerator
      cbData,
      PropertyHandlerFlags::kNonMasking);
  obj_tmpl->SetHandler(config);
}

}
