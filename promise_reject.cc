#include "_cgo_export.h"

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-promise.h"

#include "context.h"
#include "isolate-macros.h"
#include "promise_reject.h"
#include "value.h"

using namespace v8;

static void goPromiseRejectTrampoline(PromiseRejectMessage message) {
  Local<Promise> promise = message.GetPromise();
  Isolate* iso = promise->GetIsolate();

  Locker locker(iso);
  HandleScope handle_scope(iso);

  int event = static_cast<int>(message.GetEvent());

  m_ctx* ctx = static_cast<m_ctx*>(iso->GetData(0));
  if (ctx == nullptr) {
    goPromiseRejectCallback(
        reinterpret_cast<uintptr_t>(iso), event,
        nullptr, nullptr);
    return;
  }

  m_value* promise_val = new m_value;
  promise_val->id = 0;
  promise_val->iso = iso;
  promise_val->ctx = ctx;
  promise_val->ptr = Global<Value>(iso, promise);

  ValuePtr value_ptr = nullptr;
  Local<Value> val = message.GetValue();
  if (!val.IsEmpty()) {
    m_value* val_wrapper = new m_value;
    val_wrapper->id = 0;
    val_wrapper->iso = iso;
    val_wrapper->ctx = ctx;
    val_wrapper->ptr = Global<Value>(iso, val);
    value_ptr = tracked_value(ctx, val_wrapper);
  }

  goPromiseRejectCallback(
      reinterpret_cast<uintptr_t>(iso), event,
      tracked_value(ctx, promise_val), value_ptr);
}

extern "C" {

void IsolateSetPromiseRejectCallback(IsolatePtr iso) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  iso->SetPromiseRejectCallback(goPromiseRejectTrampoline);
}

}
