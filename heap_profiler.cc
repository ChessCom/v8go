#include <cstring>
#include <string>

#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-profiler.h"

#include "heap_profiler.h"

using namespace v8;

class StringOutputStream : public OutputStream {
 public:
  void EndOfStream() override {}
  int GetChunkSize() override { return 65536; }
  WriteResult WriteAsciiChunk(char* data, int size) override {
    buffer_.append(data, size);
    return kContinue;
  }
  WriteResult WriteHeapStatsChunk(HeapStatsUpdate* data, int count) override {
    return kContinue;
  }
  const std::string& str() const { return buffer_; }

 private:
  std::string buffer_;
};

extern "C" {

HeapSnapshotData IsolateTakeHeapSnapshot(IsolatePtr iso) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  HeapProfiler* profiler = iso->GetHeapProfiler();
  if (profiler == nullptr) {
    return HeapSnapshotData{nullptr, 0};
  }

  const HeapSnapshot* snapshot = profiler->TakeHeapSnapshot();
  if (snapshot == nullptr) {
    return HeapSnapshotData{nullptr, 0};
  }

  StringOutputStream stream;
  snapshot->Serialize(&stream, HeapSnapshot::kJSON);

  const std::string& s = stream.str();
  char* copy = static_cast<char*>(malloc(s.size()));
  memcpy(copy, s.data(), s.size());

  const_cast<HeapSnapshot*>(snapshot)->Delete();

  return HeapSnapshotData{copy, s.size()};
}

void HeapSnapshotDataFree(HeapSnapshotData snapshot) {
  free(const_cast<char*>(snapshot.data));
}

}
