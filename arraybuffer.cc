#include "_cgo_export.h"

#include "deps/include/v8-array-buffer.h"
#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"

#include "arraybuffer.h"
#include "context.h"
#include "context-macros.h"
#include "value.h"

using namespace v8;

static void externalBackingStoreDeleter(void* data, size_t length,
                                        void* deleter_data) {
  int ref = static_cast<int>(reinterpret_cast<intptr_t>(deleter_data));
  goReleaseExternalArrayBuffer(ref);
}

extern "C" {

ValuePtr NewArrayBufferFromBytes(ContextPtr ctx_ptr,
                                 const void* data,
                                 size_t byte_length) {
  LOCAL_CONTEXT(ctx_ptr);

  auto backing = ArrayBuffer::NewBackingStore(iso, byte_length);
  if (data != nullptr && byte_length > 0) {
    memcpy(backing->Data(), data, byte_length);
  }
  Local<ArrayBuffer> ab = ArrayBuffer::New(iso, std::move(backing));

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, ab);
  return tracked_value(ctx_ptr, val);
}

ValuePtr NewArrayBufferAlloc(ContextPtr ctx_ptr,
                             size_t byte_length) {
  LOCAL_CONTEXT(ctx_ptr);

  auto backing = ArrayBuffer::NewBackingStore(iso, byte_length);
  Local<ArrayBuffer> ab = ArrayBuffer::New(iso, std::move(backing));

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, ab);
  return tracked_value(ctx_ptr, val);
}

ValuePtr NewArrayBufferExternal(ContextPtr ctx_ptr,
                                void* data,
                                size_t byte_length,
                                int deleter_ref) {
  LOCAL_CONTEXT(ctx_ptr);

  auto backing = ArrayBuffer::NewBackingStore(
      data, byte_length, externalBackingStoreDeleter,
      reinterpret_cast<void*>(static_cast<intptr_t>(deleter_ref)));
  Local<ArrayBuffer> ab = ArrayBuffer::New(iso, std::move(backing));

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, ab);
  return tracked_value(ctx_ptr, val);
}

void* ArrayBufferGetData(ValuePtr ptr) {
  m_value* val = static_cast<m_value*>(ptr);
  Isolate* iso = val->iso;
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  Local<Value> value = val->ptr.Get(iso);
  Local<ArrayBuffer> ab = Local<ArrayBuffer>::Cast(value);
  return ab->Data();
}

size_t ArrayBufferGetByteLength(ValuePtr ptr) {
  m_value* val = static_cast<m_value*>(ptr);
  Isolate* iso = val->iso;
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  Local<Value> value = val->ptr.Get(iso);
  Local<ArrayBuffer> ab = Local<ArrayBuffer>::Cast(value);
  return ab->ByteLength();
}

BackingStorePtr ArrayBufferGetBackingStore(ValuePtr ptr) {
  m_value* val = static_cast<m_value*>(ptr);
  Isolate* iso = val->iso;
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  Local<Value> value = val->ptr.Get(iso);
  Local<ArrayBuffer> ab = Local<ArrayBuffer>::Cast(value);
  auto backing_store = ab->GetBackingStore();
  auto proxy = new v8BackingStore(std::move(backing_store));
  return proxy;
}

}
