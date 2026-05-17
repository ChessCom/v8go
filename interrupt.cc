#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "interrupt.h"

using namespace v8;

static void terminateOnInterrupt(Isolate* iso, void* data) {
  (void)data;
  iso->TerminateExecution();
}

extern "C" {

void IsolateRequestInterruptTerminate(IsolatePtr iso) {
  iso->RequestInterrupt(terminateOnInterrupt, nullptr);
}

void IsolateSetIdle(IsolatePtr iso, int is_idle) {
  iso->SetIdle(is_idle != 0);
}

}
