#include "deps/include/v8-local-handle.h"
#include "deps/include/v8-object.h"

#include <stdint.h>

using namespace v8;

static int32_t FastAddInt32(Local<Object> recv, int32_t a, int32_t b) {
  return a + b;
}

extern "C" {

void* v8go_test_FastAddInt32Addr() {
  return reinterpret_cast<void*>(&FastAddInt32);
}

}
