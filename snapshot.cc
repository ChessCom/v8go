#include <cstdlib>
#include <cstring>

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-snapshot.h"

#include "context.h"
#include "function_template.h"
#include "isolate.h"
#include "module.h"
#include "snapshot.h"
#include "template.h"

using namespace v8;

// default_allocator is owned by isolate.cc and initialised by Init().
extern ArrayBuffer::Allocator* default_allocator;

extern "C" {

// Forward-declared in isolate.cc; the heap-limit watchdog used by
// regular isolates is reused for snapshot-consumer isolates.
size_t NearMemoryLimitCallback(void* data, size_t current_heap_limit,
                               size_t initial_heap_limit);


SnapshotCreatorPtr NewSnapshotCreator(const intptr_t* external_references,
                                      const char* existing_blob_data,
                                      int existing_blob_length) {
  StartupData* existing = nullptr;
  if (existing_blob_data != nullptr && existing_blob_length > 0) {
    existing = new StartupData();
    existing->data = existing_blob_data;
    existing->raw_size = existing_blob_length;
  }

  Isolate::CreateParams params;
  params.array_buffer_allocator = default_allocator;
  if (external_references != nullptr) {
    params.external_references = external_references;
  }
  if (existing != nullptr) {
    params.snapshot_blob = existing;
  }

  SnapshotCreator* creator = new SnapshotCreator(params);

  // The SnapshotCreator constructor enters the isolate; we do NOT need
  // (and must not add) an extra Locker/Isolate::Scope here.
  Isolate* iso = creator->GetIsolate();

  // Install the m_ctx scaffolding used by NewIsolate so the Go wrapper
  // can interoperate. We deliberately skip
  // SetCaptureStackTraceForUncaughtExceptions and AddNearHeapLimitCallback
  // during snapshot construction: the former adds per-isolate state the
  // serializer does not need to capture, and the latter installs a
  // process-wide callback whose lifetime outlives the snapshot creator
  // and can fire against an unrelated isolate, corrupting V8's per-iso
  // bookkeeping.
  m_ctx* ctx = new m_ctx;
  ctx->iso = iso;
  // SnapshotCreator-owned isolates opt into template tracking so
  // CreateBlob can drain Global<Template> handles before serialisation.
  ctx->track_templates = true;
  iso->SetData(0, ctx);
  // Slot 1 carries the "existing blob StartupData pointer" so the
  // wrapper can release it symmetrically on Dispose.
  iso->SetData(1, reinterpret_cast<void*>(existing));

  return creator;
}

IsolatePtr SnapshotCreatorGetIsolate(SnapshotCreatorPtr p) {
  if (p == nullptr) {
    return nullptr;
  }
  return p->GetIsolate();
}

size_t SnapshotCreatorAddContext(SnapshotCreatorPtr p, ContextPtr ctx) {
  if (p == nullptr || ctx == nullptr) {
    return static_cast<size_t>(-1);
  }
  Isolate* iso = ctx->iso;
  HandleScope handle_scope(iso);
  Local<Context> local_ctx = ctx->ptr.Get(iso);

  // Stash the embedder-side m_ctx pointer in slot 2 so CreateBlob can
  // walk its Globals and Reset() them before serialisation. Without this
  // V8 aborts with "global handle not serialized".
  iso->SetData(2, reinterpret_cast<void*>(ctx));

  return p->AddContext(local_ctx);
}

// SnapshotCreatorReleaseEmbedderHandles walks every wrapper-managed Global<>
// reachable from the m_ctx scaffolding and Reset()s it. v8::SnapshotCreator
// refuses to serialise an isolate that still has live embedder Globals, so
// the wrapper has to actively drain them. After this call the wrapper's
// references are no longer usable, which mirrors the lifecycle published
// to the Go side: SnapshotCreator.CreateBlob consumes both the creator and
// every value handed out from it.
static void SnapshotCreatorReleaseEmbedderHandles(Isolate* iso, m_ctx* ctx) {
  if (ctx == nullptr) {
    return;
  }
  // Drop the Persistent<Context> first; this prevents iteration of
  // ctx->vals from holding any stale references after we reset them.
  ctx->ptr.Reset();
  for (auto& kv : ctx->vals) {
    m_value* v = kv.second;
    v->ptr.Reset();
  }
  for (m_unboundScript* us : ctx->unboundScripts) {
    us->ptr.Reset();
  }
  for (m_template* tmpl : ctx->templates) {
    tmpl->ptr.Reset();
  }
  // Release any module handles that the Go caller did not explicitly
  // Close() before CreateBlob. Without this V8 aborts on serialisation.
  for (m_module* mod : ctx->modules) {
    if (mod != nullptr) {
      mod->ptr.Reset();
    }
  }
}

StartupBlob SnapshotCreatorCreateBlob(SnapshotCreatorPtr p,
                                      FunctionCodeHandling fch) {
  StartupBlob out = {nullptr, 0};
  if (p == nullptr) {
    return out;
  }

  // Drain any pending m_ctx structure attached to the isolate before we
  // hand it over for serialisation. CreateBlob disposes the isolate, so
  // anything we hold in iso->GetData(0) needs to be released here.
  Isolate* iso = p->GetIsolate();
  m_ctx* slot_ctx = static_cast<m_ctx*>(iso->GetData(0));
  iso->SetData(0, nullptr);
  StartupData* slot_blob = static_cast<StartupData*>(iso->GetData(1));
  iso->SetData(1, nullptr);
  m_ctx* embedder_ctx = static_cast<m_ctx*>(iso->GetData(2));
  iso->SetData(2, nullptr);

  // V8 requires SetDefaultContext to have been called before CreateBlob.
  // We install a fresh empty context as the default so the embedder's
  // context (registered via AddContext) is recoverable as additional
  // context index 0. SnapshotCreator already entered the isolate; we
  // only need a HandleScope.
  {
    HandleScope handle_scope(iso);
    Local<Context> default_ctx = Context::New(iso);
    p->SetDefaultContext(default_ctx);
  }

  // Release every Global<> the wrapper has held on the embedder side.
  // After this point the wrapper's m_value pointers (and the Go *Value
  // wrappers backed by them) must NOT be dereferenced; the Go side has
  // already cleared its references in SnapshotCreator.CreateBlob.
  SnapshotCreatorReleaseEmbedderHandles(iso, slot_ctx);
  if (embedder_ctx != nullptr && embedder_ctx != slot_ctx) {
    SnapshotCreatorReleaseEmbedderHandles(iso, embedder_ctx);
  }

  SnapshotCreator::FunctionCodeHandling handling =
      fch == 1 ? SnapshotCreator::FunctionCodeHandling::kClear
               : SnapshotCreator::FunctionCodeHandling::kKeep;
  StartupData data = p->CreateBlob(handling);

  auto free_ctx = [](m_ctx* c) {
    if (c == nullptr) return;
    for (auto& kv : c->vals) {
      delete kv.second;
    }
    for (m_unboundScript* us : c->unboundScripts) {
      delete us;
    }
    for (m_module* mod : c->modules) {
      if (mod != nullptr) {
        delete mod;
      }
    }
    delete c;
  };
  free_ctx(slot_ctx);
  if (embedder_ctx != slot_ctx) {
    free_ctx(embedder_ctx);
  }
  if (slot_blob != nullptr) {
    delete slot_blob;
  }

  out.data = data.data;
  out.raw_size = data.raw_size;
  return out;
}

void SnapshotCreatorFreeBlob(StartupBlob blob) {
  if (blob.data == nullptr) {
    return;
  }
  delete[] blob.data;
}

void SnapshotCreatorDispose(SnapshotCreatorPtr p) {
  if (p == nullptr) {
    return;
  }
  // The v8::SnapshotCreator destructor disposes the isolate it owns.
  // The iso has already been put through CreateBlob (or never used);
  // either way the destructor walks the iso's bookkeeping. We must not
  // delete creator while another thread is inside V8 (the deser mutex
  // on the Go side guarantees that). We also leak the creator if it
  // owns an iso that V8's teardown crashes on — we'd rather lose a few
  // hundred bytes than abort the host process. The Go wrapper makes
  // sure Dispose is called from a single goroutine.
  delete p;
}

intptr_t v8go_FunctionTemplateCallback_addr(void) {
  return reinterpret_cast<intptr_t>(&FunctionTemplateCallback);
}

IsolatePtr v8go_NewIsolateWithSnapshotAndRefs(
    IsolateConstraintsPtr constraints,
    const char* snapshot_data,
    int snapshot_length,
    const intptr_t* external_references) {
  Isolate::CreateParams params;
  params.array_buffer_allocator = default_allocator;
  if (external_references != nullptr) {
    params.external_references = external_references;
  }

  if (constraints != nullptr) {
    ResourceConstraints rc;
    rc.ConfigureDefaultsFromHeapSize(
        constraints->initial_heap_size_in_bytes,
        constraints->maximum_heap_size_in_bytes);
    params.constraints = rc;
  }

  StartupData* blob = nullptr;
  if (snapshot_data != nullptr && snapshot_length > 0) {
    blob = new StartupData();
    blob->data = snapshot_data;
    blob->raw_size = snapshot_length;
    params.snapshot_blob = blob;
  }

  Isolate* iso = Isolate::New(params);
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  iso->SetCaptureStackTraceForUncaughtExceptions(true);
  // Match isolate.cc's near-heap-limit safety net.
  iso->AddNearHeapLimitCallback(NearMemoryLimitCallback, iso);

  m_ctx* ctx = new m_ctx;
  ctx->iso = iso;
  ctx->ptr.Reset(iso, Context::New(iso));
  iso->SetData(0, ctx);
  iso->SetData(1, reinterpret_cast<void*>(blob));

  return iso;
}

}  // extern "C"
