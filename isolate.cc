#include "deps/include/v8-context.h"
#include "deps/include/v8-initialization.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-platform.h"

#include "context.h"
#include "isolate.h"
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

static void ConfigureIsolate(Isolate* iso) {
  iso->SetCaptureStackTraceForUncaughtExceptions(true);
  iso->AddNearHeapLimitCallback(NearMemoryLimitCallback, iso);
}

IsolatePtr NewIsolate(IsolateConstraintsPtr constraints) {
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

  ConfigureIsolate(iso);

  m_ctx* ctx = new m_ctx;
  ctx->ptr.Reset(iso, Context::New(iso));
  ctx->iso = iso;
  iso->SetData(0, ctx);

  return iso;
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

  ConfigureIsolate(iso);

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
}
