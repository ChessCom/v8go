#ifndef V8GO_SNAPSHOT_H
#define V8GO_SNAPSHOT_H

#include "context.h"
#include "isolate.h"

#ifdef __cplusplus

namespace v8 {
class SnapshotCreator;
}
typedef v8::SnapshotCreator v8SnapshotCreator;

extern "C" {
#else
typedef struct v8SnapshotCreator v8SnapshotCreator;
#endif

#include <stddef.h>
#include <stdint.h>

typedef v8SnapshotCreator* SnapshotCreatorPtr;

// StartupBlob owns a heap-allocated buffer produced by v8::SnapshotCreator.
// The caller must call SnapshotCreatorFreeBlob to release the buffer.
typedef struct {
  const char* data;
  int raw_size;
} StartupBlob;

// FunctionCodeHandling mirrors v8::SnapshotCreator::FunctionCodeHandling.
// 0 == kKeep, 1 == kClear.
typedef int FunctionCodeHandling;

// NewSnapshotCreator creates a SnapshotCreator with the given 0-terminated
// external_references array. The array must remain valid for the lifetime
// of every isolate that consumes the resulting blob, so callers should keep
// it pinned in process-static memory. existing_blob may be NULL.
extern SnapshotCreatorPtr NewSnapshotCreator(const intptr_t* external_references,
                                             const char* existing_blob_data,
                                             int existing_blob_length);

// SnapshotCreatorGetIsolate returns the SnapshotCreator-owned isolate. It is
// already initialised with the same m_ctx scaffolding as NewIsolate so the
// Go wrapper can run scripts against it.
extern IsolatePtr SnapshotCreatorGetIsolate(SnapshotCreatorPtr p);

// SnapshotCreatorAddContext registers the embedder context that
// Context::FromSnapshot(iso, 0) will return at restore time. Must be called
// exactly once before SnapshotCreatorCreateBlob. The provided context must
// belong to the creator's isolate. Returns the V8 context index (0 for the
// first added context).
extern size_t SnapshotCreatorAddContext(SnapshotCreatorPtr p, ContextPtr ctx);

// SnapshotCreatorCreateBlob serialises the heap into a StartupBlob. After
// this call the creator's isolate is disposed and SnapshotCreatorDispose
// must be invoked to release the SnapshotCreator itself.
extern StartupBlob SnapshotCreatorCreateBlob(SnapshotCreatorPtr p,
                                             FunctionCodeHandling fch);

// SnapshotCreatorFreeBlob releases the buffer returned by CreateBlob.
extern void SnapshotCreatorFreeBlob(StartupBlob blob);

// SnapshotCreatorDispose releases the SnapshotCreator. Must be called even
// when CreateBlob was successful.
extern void SnapshotCreatorDispose(SnapshotCreatorPtr p);

// v8go_FunctionTemplateCallback_addr returns the address of the C++
// FunctionTemplateCallback trampoline as an intptr_t. The Go-side external
// reference registry installs this as the first built-in entry so any
// snapshot containing Go-backed FunctionTemplates can be restored.
extern intptr_t v8go_FunctionTemplateCallback_addr(void);

// v8go_NewIsolateWithSnapshotAndRefs is the snapshot-aware isolate factory
// that wires in a 0-terminated external_references array.
extern IsolatePtr v8go_NewIsolateWithSnapshotAndRefs(
    IsolateConstraintsPtr constraints,
    const char* snapshot_data,
    int snapshot_length,
    const intptr_t* external_references);

#ifdef __cplusplus
}  // extern "C"
#endif

#endif  // V8GO_SNAPSHOT_H
