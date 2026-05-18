#include "deps/include/v8-context.h"
#include "deps/include/v8-function.h"
#include "deps/include/v8-initialization.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-microtask.h"
#include "deps/include/v8-platform.h"
#include "deps/include/v8-callbacks.h"

#include "context.h"
#include "isolate.h"
#include "value.h"
#include "libplatform/libplatform.h"

using namespace v8;

auto default_platform = platform::NewDefaultPlatform();
ArrayBuffer::Allocator* default_allocator;

extern "C" {

/********** Isolate **********/

#define ISOLATE_SCOPE(iso)           \
  Locker locker(iso);                \
  Isolate::Scope isolate_scope(iso); \
  HandleScope handle_scope(iso);

void Init() {
#ifdef _WIN32
  V8::InitializeExternalStartupData(".");
#endif
  V8::InitializePlatform(default_platform.get());
  V8::Initialize();

  default_allocator = ArrayBuffer::Allocator::NewDefaultAllocator();
  return;
}

size_t NearMemoryLimitCallback(void* data, size_t current_heap_limit, size_t initial_heap_limit)
{
  auto iso = static_cast<Isolate*>(data);
  iso->TerminateExecution();

  // if we return the initial heap limit, the VM will crash, so here we give it room to exit gracefully
  return current_heap_limit * 2;
}

static void ConfigureIsolate(Isolate* iso, bool add_heap_limit_cb) {
  iso->SetCaptureStackTraceForUncaughtExceptions(true);
  if (add_heap_limit_cb) {
    iso->AddNearHeapLimitCallback(NearMemoryLimitCallback, iso);
  }
}

static IsolatePtr NewIsolateInternal(IsolateConstraintsPtr constraints, bool add_heap_limit_cb) {
  Isolate::CreateParams params;
  params.array_buffer_allocator = default_allocator;

  if (constraints != nullptr) {
    ResourceConstraints rc;
    rc.ConfigureDefaultsFromHeapSize(
      constraints->initial_heap_size_in_bytes,
      constraints->maximum_heap_size_in_bytes
    );
    params.constraints = rc;
  }

  Isolate* iso = Isolate::New(params);
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  ConfigureIsolate(iso, add_heap_limit_cb);

  m_ctx* ctx = new m_ctx;
  ctx->ptr.Reset(iso, Context::New(iso));
  ctx->iso = iso;
  iso->SetData(0, ctx);

  return iso;
}

IsolatePtr NewIsolate(IsolateConstraintsPtr constraints) {
  return NewIsolateInternal(constraints, true);
}

IsolatePtr NewIsolateNoDefaultHeapCB(IsolateConstraintsPtr constraints) {
  return NewIsolateInternal(constraints, false);
}

IsolatePtr NewIsolateWithSnapshot(IsolateConstraintsPtr constraints,
                                  const char* snapshot_data,
                                  int snapshot_length) {
  Isolate::CreateParams params;
  params.array_buffer_allocator = default_allocator;

  if (constraints != nullptr) {
    ResourceConstraints rc;
    rc.ConfigureDefaultsFromHeapSize(
      constraints->initial_heap_size_in_bytes,
      constraints->maximum_heap_size_in_bytes
    );
    params.constraints = rc;
  }

  // Heap-allocate the StartupData so V8 can reference it throughout
  // the isolate's lifetime (it keeps a pointer internally).
  auto* blob = new StartupData();
  blob->data = snapshot_data;
  blob->raw_size = snapshot_length;
  params.snapshot_blob = blob;

  Isolate* iso = Isolate::New(params);
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  ConfigureIsolate(iso, true);

  m_ctx* ctx = new m_ctx;
  ctx->iso = iso;
  iso->SetData(0, ctx);
  iso->SetData(1, reinterpret_cast<void*>(blob)); // snapshot blob ptr for cleanup

  return iso;
}

void IsolateDisposeSnapshot(IsolatePtr iso) {
  auto* blob = static_cast<StartupData*>(iso->GetData(1));
  if (blob != nullptr) {
    delete blob;
    iso->SetData(1, nullptr);
  }
}

int IsolateHasSnapshot(IsolatePtr iso) {
  return iso->GetData(1) != nullptr ? 1 : 0;
}

void IsolatePerformMicrotaskCheckpoint(IsolatePtr iso) {
  ISOLATE_SCOPE(iso)
  iso->PerformMicrotaskCheckpoint();
}

void IsolateDispose(IsolatePtr iso) {
  if (iso == nullptr) {
    return;
  }
  auto ctx = static_cast<m_ctx*>(iso->GetData(0));
  ContextFree(ctx);

  iso->Dispose();
}

void IsolateTerminateExecution(IsolatePtr iso) {
  iso->TerminateExecution();
}

int IsolateIsExecutionTerminating(IsolatePtr iso) {
  return iso->IsExecutionTerminating();
}

IsolateHStatistics IsolationGetHeapStatistics(IsolatePtr iso) {
  if (iso == nullptr) {
    return IsolateHStatistics{0};
  }
  v8::HeapStatistics hs;
  iso->GetHeapStatistics(&hs);

  return IsolateHStatistics{hs.total_heap_size(),
                            hs.total_heap_size_executable(),
                            hs.total_physical_size(),
                            hs.total_available_size(),
                            hs.used_heap_size(),
                            hs.heap_size_limit(),
                            hs.malloced_memory(),
                            hs.external_memory(),
                            hs.peak_malloced_memory(),
                            hs.number_of_native_contexts(),
                            hs.number_of_detached_contexts()};
}

// ChessCom: GC and memory pressure APIs

void IsolateLowMemoryNotification(IsolatePtr iso) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  iso->LowMemoryNotification();
}

void IsolateMemoryPressureNotification(IsolatePtr iso, int level) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  iso->MemoryPressureNotification(static_cast<MemoryPressureLevel>(level));
}

// Thread-safe per V8 spec — no Locker needed.
void IsolateCancelTerminateExecution(IsolatePtr iso) {
  iso->CancelTerminateExecution();
}

void IsolateRequestGarbageCollectionForTesting(IsolatePtr iso, int type) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  iso->RequestGarbageCollectionForTesting(
      static_cast<Isolate::GarbageCollectionType>(type));
}

void IsolateContextDisposedNotification(IsolatePtr iso, int dependant_context) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  if (dependant_context) {
    iso->ContextDisposedNotification(ContextDependants::kSomeDependants);
  } else {
    iso->ContextDisposedNotification(ContextDependants::kNoDependants);
  }
}

int64_t IsolateAdjustExternalMemory(IsolatePtr iso, int64_t change_in_bytes) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
  return iso->AdjustAmountOfExternalAllocatedMemory(change_in_bytes);
#pragma GCC diagnostic pop
}

void IsolateSetMicrotasksPolicy(IsolatePtr iso, int policy) {
  iso->SetMicrotasksPolicy(static_cast<MicrotasksPolicy>(policy));
}

void IsolateEnqueueMicrotask(IsolatePtr iso, ValuePtr fn_ptr) {
  ISOLATE_SCOPE(iso);
  m_value* val = static_cast<m_value*>(fn_ptr);
  m_ctx* ctx = val->ctx;
  Local<Context> local_ctx = ctx->ptr.Get(iso);
  Context::Scope context_scope(local_ctx);
  Local<Value> local_val = val->ptr.Get(iso);
  Local<Function> fn = Local<Function>::Cast(local_val);
  iso->EnqueueMicrotask(fn);
}
}
