#include "_cgo_export.h"

#include "deps/include/v8-callbacks.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "gc_callback.h"

using namespace v8;

static void goGCPrologueTrampoline(Isolate* iso, GCType type,
                                   GCCallbackFlags flags, void* data) {
  (void)flags;
  (void)data;
  goGCPrologueCallback(reinterpret_cast<uintptr_t>(iso),
                        static_cast<int>(type));
}

static void goGCEpilogueTrampoline(Isolate* iso, GCType type,
                                   GCCallbackFlags flags, void* data) {
  (void)flags;
  (void)data;
  goGCEpilogueCallback(reinterpret_cast<uintptr_t>(iso),
                        static_cast<int>(type));
}

extern "C" {

void IsolateAddGCPrologueCallback(IsolatePtr iso) {
  iso->AddGCPrologueCallback(goGCPrologueTrampoline, nullptr, kGCTypeAll);
}

void IsolateRemoveGCPrologueCallback(IsolatePtr iso) {
  iso->RemoveGCPrologueCallback(goGCPrologueTrampoline, nullptr);
}

void IsolateAddGCEpilogueCallback(IsolatePtr iso) {
  iso->AddGCEpilogueCallback(goGCEpilogueTrampoline, nullptr, kGCTypeAll);
}

void IsolateRemoveGCEpilogueCallback(IsolatePtr iso) {
  iso->RemoveGCEpilogueCallback(goGCEpilogueTrampoline, nullptr);
}

}
