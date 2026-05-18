#include "_cgo_export.h"

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-script.h"

#include "context.h"
#include "context-macros.h"
#include "module.h"
#include "value.h"

using namespace v8;

static MaybeLocal<Module> resolveModuleTrampoline(
    Local<Context> context,
    Local<String> specifier,
    Local<FixedArray> import_attributes,
    Local<Module> referrer) {
  Isolate* iso = context->GetIsolate();
  int ctx_ref = context->GetEmbedderData(1).As<Integer>()->Value();

  String::Utf8Value spec(iso, specifier);
  int referrer_hash = referrer->GetIdentityHash();

  ModulePtr result = goResolveModuleCallback(ctx_ref, *spec, referrer_hash);
  if (result == nullptr) {
    iso->ThrowError("Module resolution failed: module not found");
    return MaybeLocal<Module>();
  }

  return result->ptr.Get(iso);
}

extern "C" {

RtnModule CompileESModule(ContextPtr ctx_ptr,
                          const char* source,
                          const char* origin) {
  LOCAL_CONTEXT(ctx_ptr);
  RtnModule rtn = {};

  Local<String> src =
      String::NewFromUtf8(iso, source, NewStringType::kNormal).ToLocalChecked();
  Local<String> orig =
      String::NewFromUtf8(iso, origin, NewStringType::kNormal).ToLocalChecked();

  ScriptOrigin script_origin(orig,
                             0,      // line offset
                             0,      // column offset
                             false,  // is shared cross-origin
                             -1,     // script id
                             {},     // source map URL
                             false,  // is opaque
                             false,  // is WASM
                             true);  // is module

  ScriptCompiler::Source compiler_source(src, script_origin);
  Local<Module> module;
  if (!ScriptCompiler::CompileModule(iso, &compiler_source).ToLocal(&module)) {
    rtn.error = ExceptionError(try_catch, iso, local_ctx);
    return rtn;
  }

  m_module* mod = new m_module;
  mod->ptr.Reset(iso, module);
  mod->iso = iso;
  mod->tracked = false;
  rtn.ptr = mod;

  // Track module for auto-release during SnapshotCreator.CreateBlob.
  m_ctx* ctx = static_cast<m_ctx*>(iso->GetData(0));
  if (ctx != nullptr && ctx->track_templates) {
    mod->tracked = true;
    ctx->modules.push_back(mod);
  }

  return rtn;
}

int ModuleGetStatus(IsolatePtr iso, ModulePtr mod) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);
  return static_cast<int>(mod->ptr.Get(iso)->GetStatus());
}

int ModuleGetRequestsLength(IsolatePtr iso, ModulePtr mod) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);
  return mod->ptr.Get(iso)->GetModuleRequests()->Length();
}

const char* ModuleGetRequest(ContextPtr ctx_ptr, ModulePtr mod, int index) {
  LOCAL_CONTEXT(ctx_ptr);
  Local<Module> local_mod = mod->ptr.Get(iso);
  Local<FixedArray> requests = local_mod->GetModuleRequests();
  Local<Data> data = requests->Get(local_ctx, index);
  Local<ModuleRequest> req = data.As<ModuleRequest>();
  String::Utf8Value spec(iso, req->GetSpecifier());
  return strdup(*spec);
}

int ModuleGetIdentityHash(IsolatePtr iso, ModulePtr mod) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);
  return mod->ptr.Get(iso)->GetIdentityHash();
}

int ModuleInstantiate(ContextPtr ctx_ptr, ModulePtr mod) {
  LOCAL_CONTEXT(ctx_ptr);

  Local<Module> local_mod = mod->ptr.Get(iso);
  Maybe<bool> result = local_mod->InstantiateModule(
      local_ctx, resolveModuleTrampoline);

  if (result.IsNothing() || !result.FromJust()) {
    return 0;
  }
  return 1;
}

RtnValue ModuleEvaluate(ContextPtr ctx_ptr, ModulePtr mod) {
  LOCAL_CONTEXT(ctx_ptr);
  RtnValue rtn = {};

  Local<Module> local_mod = mod->ptr.Get(iso);
  Local<Value> result;
  if (!local_mod->Evaluate(local_ctx).ToLocal(&result)) {
    rtn.error = ExceptionError(try_catch, iso, local_ctx);
    return rtn;
  }

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, result);
  rtn.value = tracked_value(ctx_ptr, val);
  return rtn;
}

ValuePtr ModuleGetNamespace(ContextPtr ctx_ptr, ModulePtr mod) {
  LOCAL_CONTEXT(ctx_ptr);

  Local<Module> local_mod = mod->ptr.Get(iso);
  Local<Value> ns = local_mod->GetModuleNamespace();

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx_ptr;
  val->ptr.Reset(iso, ns);
  return tracked_value(ctx_ptr, val);
}

void ModuleFree(ModulePtr mod) {
  if (mod == nullptr) return;
  mod->ptr.Reset();
  if (mod->tracked) {
    // Module is tracked by a SnapshotCreator context. Null out the
    // tracking entry so the release code skips it; the struct will be
    // freed by SnapshotCreatorReleaseEmbedderHandles.
    m_ctx* ctx = static_cast<m_ctx*>(mod->iso->GetData(0));
    if (ctx != nullptr) {
      for (auto& entry : ctx->modules) {
        if (entry == mod) {
          entry = nullptr;
          break;
        }
      }
    }
  }
  delete mod;
}

}
