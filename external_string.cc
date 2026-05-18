#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-primitive.h"

#include "context.h"
#include "context-macros.h"
#include "external_string.h"
#include "value.h"

using namespace v8;

class GoExternalOneByteStringResource
    : public String::ExternalOneByteStringResource {
 public:
  GoExternalOneByteStringResource(const char* data, size_t length)
      : data_(data), length_(length) {}

  ~GoExternalOneByteStringResource() override {}

  const char* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  const char* data_;
  size_t length_;
};

extern "C" {

ValuePtr NewExternalOneByteString(ContextPtr ctx_ptr,
                                  const char* data,
                                  size_t length) {
  LOCAL_CONTEXT(ctx_ptr);

  auto* resource = new GoExternalOneByteStringResource(data, length);
  MaybeLocal<String> maybe = String::NewExternalOneByte(iso, resource);
  Local<String> str;
  if (!maybe.ToLocal(&str)) {
    delete resource;
    return nullptr;
  }

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, str);
  return tracked_value(ctx_ptr, val);
}

}
