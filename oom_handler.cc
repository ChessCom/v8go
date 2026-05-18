#include "_cgo_export.h"

#include "deps/include/v8-callbacks.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "oom_handler.h"

using namespace v8;

static void goOOMTrampoline(const char* location,
                            const OOMDetails& details) {
  // V8 does not pass the Isolate* directly to the OOM handler.
  // We propagate via thread-local set just before the call that may OOM.
  // However, V8's SetOOMErrorHandler is per-isolate, so we use the
  // Isolate::TryGetCurrent() API which returns the entered isolate.
  Isolate* iso = Isolate::TryGetCurrent();
  if (iso == nullptr) {
    return;
  }
  goOOMErrorCallback(reinterpret_cast<uintptr_t>(iso), const_cast<char*>(location),
                     details.is_heap_oom ? 1 : 0);
}

extern "C" {

void IsolateSetOOMErrorHandler(IsolatePtr iso) {
  iso->SetOOMErrorHandler(goOOMTrampoline);
}

void IsolateClearOOMErrorHandler(IsolatePtr iso) {
  iso->SetOOMErrorHandler(nullptr);
}

}
