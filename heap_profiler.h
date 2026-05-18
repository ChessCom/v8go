#ifndef V8GO_HEAP_PROFILER_H
#define V8GO_HEAP_PROFILER_H

#include "isolate.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  const char* data;
  size_t length;
} HeapSnapshotData;

extern HeapSnapshotData IsolateTakeHeapSnapshot(IsolatePtr iso_ptr);
extern void HeapSnapshotDataFree(HeapSnapshotData snapshot);

#ifdef __cplusplus
}  // extern "C"
#endif
#endif
