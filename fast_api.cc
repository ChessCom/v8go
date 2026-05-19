#include "deps/include/v8-fast-api-calls.h"
#include "fast_api.h"

using namespace v8;

static CTypeInfo::Type toCTypeInfoType(CTypeInfoType t) {
  switch (t) {
    case kCTypeVoid:
      return CTypeInfo::Type::kVoid;
    case kCTypeBool:
      return CTypeInfo::Type::kBool;
    case kCTypeUint8:
      return CTypeInfo::Type::kUint8;
    case kCTypeInt32:
      return CTypeInfo::Type::kInt32;
    case kCTypeUint32:
      return CTypeInfo::Type::kUint32;
    case kCTypeInt64:
      return CTypeInfo::Type::kInt64;
    case kCTypeUint64:
      return CTypeInfo::Type::kUint64;
    case kCTypeFloat32:
      return CTypeInfo::Type::kFloat32;
    case kCTypeFloat64:
      return CTypeInfo::Type::kFloat64;
    case kCTypePointer:
      return CTypeInfo::Type::kPointer;
    case kCTypeV8Value:
      return CTypeInfo::Type::kV8Value;
    case kCTypeSeqOneByteString:
      return CTypeInfo::Type::kSeqOneByteString;
    default:
      return CTypeInfo::Type::kVoid;
  }
}

extern "C" {

CFunctionInfoPtr BuildCFunctionInfo(CTypeInfoType return_type,
                                    const CTypeInfoType* arg_types,
                                    int arg_count) {
  CTypeInfo ret_info(toCTypeInfoType(return_type));

  CTypeInfo* args = new CTypeInfo[arg_count];
  for (int i = 0; i < arg_count; i++) {
    args[i] = CTypeInfo(toCTypeInfoType(arg_types[i]));
  }

  CFunctionInfo* info = new CFunctionInfo(ret_info, arg_count, args);
  return static_cast<CFunctionInfoPtr>(info);
}

void FreeCFunctionInfo(CFunctionInfoPtr ptr) {
  if (ptr == nullptr) return;
  auto* info = static_cast<CFunctionInfo*>(ptr);
  delete info;
}

}
