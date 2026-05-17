#include "_cgo_export.h"

#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "heap_limit.h"

using namespace v8;

static size_t goNearHeapLimitTrampoline(void* data,
                                        size_t current_heap_limit,
                                        size_t initial_heap_limit) {
  auto iso = static_cast<Isolate*>(data);
  return goNearHeapLimitCallback(
      reinterpret_cast<uintptr_t>(iso), current_heap_limit, initial_heap_limit);
}

extern "C" {

void IsolateAddCustomNearHeapLimitCallback(IsolatePtr iso) {
  iso->AddNearHeapLimitCallback(goNearHeapLimitTrampoline, iso);
}

void IsolateRemoveCustomNearHeapLimitCallback(IsolatePtr iso,
                                              size_t heap_limit) {
  iso->RemoveNearHeapLimitCallback(goNearHeapLimitTrampoline, heap_limit);
}

}
