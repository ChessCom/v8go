package v8go_test

import (
	"strings"
	"testing"
	"time"

	v8 "github.com/ChessCom/v8go"
)

// --- Spike / Foundation Tests ---

func TestSnapshotESM_NamedExport(t *testing.T) {
	// Validates that CompileModule + Instantiate + Evaluate inside
	// SnapshotCreator produces a heap whose bridged globals survive
	// CreateBlob and restore. This is the gate for all ESM snapshot work.

	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`export const greeting = "hello";`, "mod.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	if ns == nil {
		t.Fatal("GetNamespace returned nil")
	}

	global := ctx.Global()
	if err := global.Set("captured", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	if len(blob) == 0 {
		t.Fatal("expected non-empty blob")
	}

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`captured.greeting`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "hello" {
		t.Fatalf("captured.greeting = %q, want %q", got, "hello")
	}
}

func TestSnapshotESM_DefaultExport(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`
		const app = {
			name: "TestApp",
			render(input) { return "<div>" + input + "</div>"; }
		};
		export default app;
	`, "app.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	global := ctx.Global()
	if err := global.Set("__module", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`__module.default.render("world")`, "probe.js")
	if err != nil {
		t.Fatalf("probe render: %v", err)
	}
	if got := val.String(); got != "<div>world</div>" {
		t.Fatalf("render = %q, want %q", got, "<div>world</div>")
	}

	nameVal, err := c.RunScript(`__module.default.name`, "probe2.js")
	if err != nil {
		t.Fatalf("probe name: %v", err)
	}
	if got := nameVal.String(); got != "TestApp" {
		t.Fatalf("name = %q, want %q", got, "TestApp")
	}
}

func TestSnapshotESM_FunctionClosure(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`
		const prefix = "Hello, ";
		const suffix = "!";
		export function greet(name) {
			return prefix + name + suffix;
		}
		export const counter = (() => {
			let n = 0;
			return { inc() { return ++n; }, get() { return n; } };
		})();
	`, "closure.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	global := ctx.Global()
	if err := global.Set("exports", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`exports.greet("V8")`, "probe.js")
	if err != nil {
		t.Fatalf("probe greet: %v", err)
	}
	if got := val.String(); got != "Hello, V8!" {
		t.Fatalf("greet = %q, want %q", got, "Hello, V8!")
	}

	val2, err := c.RunScript(`exports.counter.inc(); exports.counter.inc(); exports.counter.get()`, "probe2.js")
	if err != nil {
		t.Fatalf("probe counter: %v", err)
	}
	if got := val2.Int32(); got != 2 {
		t.Fatalf("counter.get() = %d, want 2", got)
	}
}

// --- Multi-Chunk / Dependency Tests ---

func TestSnapshotESM_MultiChunk(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	depMod, err := ctx.CompileModule(`export const multiply = (a, b) => a * b;`, "chunk-math.mjs")
	if err != nil {
		t.Fatalf("CompileModule dep: %v", err)
	}

	mainMod, err := ctx.CompileModule(`
		import { multiply } from './chunk-math.mjs';
		export const square = (n) => multiply(n, n);
		export const cube = (n) => multiply(multiply(n, n), n);
	`, "entry.mjs")
	if err != nil {
		t.Fatalf("CompileModule main: %v", err)
	}

	resolver := func(specifier string, referrerHash int) *v8.Module {
		if specifier == "./chunk-math.mjs" {
			return depMod
		}
		return nil
	}

	if err := depMod.Instantiate(resolver); err != nil {
		t.Fatalf("Instantiate dep: %v", err)
	}
	if err := mainMod.Instantiate(resolver); err != nil {
		t.Fatalf("Instantiate main: %v", err)
	}

	if _, err := depMod.Evaluate(); err != nil {
		t.Fatalf("Evaluate dep: %v", err)
	}
	if _, err := mainMod.Evaluate(); err != nil {
		t.Fatalf("Evaluate main: %v", err)
	}

	ns := mainMod.GetNamespace()
	global := ctx.Global()
	if err := global.Set("math", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	depMod.Close()
	mainMod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`math.square(7)`, "probe.js")
	if err != nil {
		t.Fatalf("probe square: %v", err)
	}
	if got := val.Int32(); got != 49 {
		t.Fatalf("square(7) = %d, want 49", got)
	}

	val2, err := c.RunScript(`math.cube(3)`, "probe2.js")
	if err != nil {
		t.Fatalf("probe cube: %v", err)
	}
	if got := val2.Int32(); got != 27 {
		t.Fatalf("cube(3) = %d, want 27", got)
	}
}

func TestSnapshotESM_ThreeChunkDiamond(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	// Diamond: entry -> A, entry -> B, A -> shared, B -> shared
	sharedMod, err := ctx.CompileModule(`export const VERSION = "1.0";`, "shared.mjs")
	if err != nil {
		t.Fatalf("CompileModule shared: %v", err)
	}

	aMod, err := ctx.CompileModule(`
		import { VERSION } from './shared.mjs';
		export const fromA = "A-" + VERSION;
	`, "a.mjs")
	if err != nil {
		t.Fatalf("CompileModule a: %v", err)
	}

	bMod, err := ctx.CompileModule(`
		import { VERSION } from './shared.mjs';
		export const fromB = "B-" + VERSION;
	`, "b.mjs")
	if err != nil {
		t.Fatalf("CompileModule b: %v", err)
	}

	entryMod, err := ctx.CompileModule(`
		import { fromA } from './a.mjs';
		import { fromB } from './b.mjs';
		export const combined = fromA + "+" + fromB;
	`, "entry.mjs")
	if err != nil {
		t.Fatalf("CompileModule entry: %v", err)
	}

	resolver := func(specifier string, referrerHash int) *v8.Module {
		switch specifier {
		case "./shared.mjs":
			return sharedMod
		case "./a.mjs":
			return aMod
		case "./b.mjs":
			return bMod
		}
		return nil
	}

	for _, m := range []*v8.Module{sharedMod, aMod, bMod, entryMod} {
		if err := m.Instantiate(resolver); err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
	}
	for _, m := range []*v8.Module{sharedMod, aMod, bMod, entryMod} {
		if _, err := m.Evaluate(); err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
	}

	ns := entryMod.GetNamespace()
	if err := ctx.Global().Set("entry", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	sharedMod.Close()
	aMod.Close()
	bMod.Close()
	entryMod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`entry.combined`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "A-1.0+B-1.0" {
		t.Fatalf("combined = %q, want %q", got, "A-1.0+B-1.0")
	}
}

// --- Stacked Snapshot Tests ---

func TestSnapshotESM_StackedOnExisting(t *testing.T) {
	// Build a base snapshot with RunScript (polyfills/globals)
	sc1 := v8.NewSnapshotCreator()
	ctx1 := sc1.Context()
	if _, err := ctx1.RunScript(`globalThis.BASE_VERSION = 42;`, "base.js"); err != nil {
		t.Fatalf("base script: %v", err)
	}
	baseBlob, err := sc1.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob base: %v", err)
	}
	sc1.Dispose()

	// Layer an ESM module on top
	sc2 := v8.NewSnapshotCreator(v8.WithExistingSnapshotBlob(baseBlob))
	ctx2 := sc2.Context()

	mod, err := ctx2.CompileModule(`
		export function getVersion() { return globalThis.BASE_VERSION; }
		export const doubled = globalThis.BASE_VERSION * 2;
	`, "overlay.mjs")
	if err != nil {
		t.Fatalf("CompileModule overlay: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	if err := ctx2.Global().Set("overlay", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}
	mod.Close()

	layeredBlob, err := sc2.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob layered: %v", err)
	}
	sc2.Dispose()

	// Restore and verify both layers
	iso := v8.NewIsolate(v8.WithSnapshotBlob(layeredBlob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`overlay.getVersion()`, "probe.js")
	if err != nil {
		t.Fatalf("probe getVersion: %v", err)
	}
	if got := val.Int32(); got != 42 {
		t.Fatalf("getVersion() = %d, want 42", got)
	}

	val2, err := c.RunScript(`overlay.doubled`, "probe2.js")
	if err != nil {
		t.Fatalf("probe doubled: %v", err)
	}
	if got := val2.Int32(); got != 84 {
		t.Fatalf("doubled = %d, want 84", got)
	}

	val3, err := c.RunScript(`BASE_VERSION`, "probe3.js")
	if err != nil {
		t.Fatalf("probe BASE_VERSION: %v", err)
	}
	if got := val3.Int32(); got != 42 {
		t.Fatalf("BASE_VERSION = %d, want 42", got)
	}
}

// --- Complex Object Graph ---

func TestSnapshotESM_ComplexObjectGraph(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`
		class Component {
			constructor(name, children) {
				this.name = name;
				this.children = children || [];
				this.props = {};
			}
			addProp(key, val) { this.props[key] = val; return this; }
			render() {
				const childHtml = this.children.map(c => c.render()).join("");
				const attrs = Object.entries(this.props)
					.map(([k, v]) => k + '="' + v + '"').join(" ");
				return "<" + this.name + (attrs ? " " + attrs : "") + ">" + childHtml + "</" + this.name + ">";
			}
		}

		const tree = new Component("div", [
			new Component("h1", []).addProp("class", "title"),
			new Component("p", [
				new Component("span", []).addProp("id", "inner")
			])
		]).addProp("id", "root");

		export { Component, tree };
	`, "components.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	if err := ctx.Global().Set("mod", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}
	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`mod.tree.render()`, "probe.js")
	if err != nil {
		t.Fatalf("probe render: %v", err)
	}
	expected := `<div id="root"><h1 class="title"></h1><p><span id="inner"></span></p></div>`
	if got := val.String(); got != expected {
		t.Fatalf("render =\n  %q\nwant:\n  %q", got, expected)
	}

	// Verify the class constructor still works after restore
	val2, err := c.RunScript(`new mod.Component("footer", []).addProp("class", "ft").render()`, "probe2.js")
	if err != nil {
		t.Fatalf("probe new Component: %v", err)
	}
	if got := val2.String(); got != `<footer class="ft"></footer>` {
		t.Fatalf("new Component render = %q", got)
	}
}

// --- Safety / Edge Case Tests ---

func TestSnapshotESM_ModuleAutoRelease(t *testing.T) {
	// Verify that forgetting to call mod.Close() does NOT crash thanks
	// to the m_ctx auto-release in SnapshotCreatorReleaseEmbedderHandles.
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`export const x = 99;`, "auto.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}
	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	if err := ctx.Global().Set("ns", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	// Intentionally NOT calling mod.Close() — auto-release should handle it
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob (without mod.Close): %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`ns.x`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.Int32(); got != 99 {
		t.Fatalf("ns.x = %d, want 99", got)
	}
}

func TestSnapshotESM_EmptyModule(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(``, "empty.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}
	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	// Bridge a marker to prove the snapshot path didn't crash
	if _, err := ctx.RunScript(`globalThis.emptyModuleOK = true;`, "mark.js"); err != nil {
		t.Fatalf("RunScript: %v", err)
	}

	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`emptyModuleOK`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !val.Boolean() {
		t.Fatal("emptyModuleOK not true")
	}
}

func TestSnapshotESM_ReExport(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	baseMod, err := ctx.CompileModule(`export const BASE = 100;`, "base.mjs")
	if err != nil {
		t.Fatalf("CompileModule base: %v", err)
	}

	reexportMod, err := ctx.CompileModule(`export { BASE } from './base.mjs';`, "reexport.mjs")
	if err != nil {
		t.Fatalf("CompileModule reexport: %v", err)
	}

	resolver := func(specifier string, referrerHash int) *v8.Module {
		if specifier == "./base.mjs" {
			return baseMod
		}
		return nil
	}

	if err := baseMod.Instantiate(resolver); err != nil {
		t.Fatalf("Instantiate base: %v", err)
	}
	if err := reexportMod.Instantiate(resolver); err != nil {
		t.Fatalf("Instantiate reexport: %v", err)
	}
	if _, err := baseMod.Evaluate(); err != nil {
		t.Fatalf("Evaluate base: %v", err)
	}
	if _, err := reexportMod.Evaluate(); err != nil {
		t.Fatalf("Evaluate reexport: %v", err)
	}

	ns := reexportMod.GetNamespace()
	if err := ctx.Global().Set("re", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}

	baseMod.Close()
	reexportMod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`re.BASE`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.Int32(); got != 100 {
		t.Fatalf("re.BASE = %d, want 100", got)
	}
}

func TestSnapshotESM_FunctionCodeClear(t *testing.T) {
	sc := v8.NewSnapshotCreator()
	ctx := sc.Context()

	mod, err := ctx.CompileModule(`
		export function expensive(n) {
			let sum = 0;
			for (let i = 0; i < n; i++) sum += i;
			return sum;
		}
	`, "expensive.mjs")
	if err != nil {
		t.Fatalf("CompileModule: %v", err)
	}

	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module {
		return nil
	}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	ns := mod.GetNamespace()
	if err := ctx.Global().Set("fn", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}
	mod.Close()

	blob, err := sc.CreateBlob(v8.FunctionCodeClear)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`fn.expensive(100)`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.Int32(); got != 4950 {
		t.Fatalf("expensive(100) = %d, want 4950", got)
	}
}

// --- Render Parity Test ---

func TestSnapshotESM_RenderParity(t *testing.T) {
	// Verify that output from a snapshot-restored module is identical
	// to output from a freshly-evaluated module.
	const source = `
		const styles = { color: "red", size: "16px" };
		export function render(tag, text) {
			return "<" + tag + " style=\"color:" + styles.color + ";font-size:" + styles.size + "\">" + text + "</" + tag + ">";
		}
	`

	// Path 1: fresh evaluation
	iso1 := v8.NewIsolate()
	ctx1 := v8.NewContext(iso1)
	mod1, err := ctx1.CompileModule(source, "parity.mjs")
	if err != nil {
		t.Fatalf("CompileModule fresh: %v", err)
	}
	if err := mod1.Instantiate(func(specifier string, referrerHash int) *v8.Module { return nil }); err != nil {
		t.Fatalf("Instantiate fresh: %v", err)
	}
	if _, err := mod1.Evaluate(); err != nil {
		t.Fatalf("Evaluate fresh: %v", err)
	}
	ns1 := mod1.GetNamespace()
	if err := ctx1.Global().Set("m", ns1); err != nil {
		t.Fatalf("global.Set fresh: %v", err)
	}
	freshResult, err := ctx1.RunScript(`m.render("p", "hello")`, "probe.js")
	if err != nil {
		t.Fatalf("fresh render: %v", err)
	}
	freshOutput := freshResult.String()
	mod1.Close()
	ctx1.Close()
	iso1.Dispose()

	// Path 2: snapshot round-trip
	sc := v8.NewSnapshotCreator()
	ctx2 := sc.Context()
	mod2, err := ctx2.CompileModule(source, "parity.mjs")
	if err != nil {
		t.Fatalf("CompileModule snap: %v", err)
	}
	if err := mod2.Instantiate(func(specifier string, referrerHash int) *v8.Module { return nil }); err != nil {
		t.Fatalf("Instantiate snap: %v", err)
	}
	if _, err := mod2.Evaluate(); err != nil {
		t.Fatalf("Evaluate snap: %v", err)
	}
	ns2 := mod2.GetNamespace()
	if err := ctx2.Global().Set("m", ns2); err != nil {
		t.Fatalf("global.Set snap: %v", err)
	}
	mod2.Close()
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	iso2 := v8.NewIsolate(v8.WithSnapshotBlob(blob))
	defer iso2.Dispose()
	ctx3 := v8.NewContext(iso2)
	defer ctx3.Close()
	snapResult, err := ctx3.RunScript(`m.render("p", "hello")`, "probe.js")
	if err != nil {
		t.Fatalf("snap render: %v", err)
	}
	snapOutput := snapResult.String()

	if freshOutput != snapOutput {
		t.Fatalf("render parity failed:\n  fresh: %q\n  snap:  %q", freshOutput, snapOutput)
	}
}

// --- Performance Regression ---

func TestSnapshotESM_ColdStartSpeedup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ESM cold-start benchmark in -short mode")
	}
	if testing.CoverMode() != "" {
		t.Skip("skipping cold-start benchmark under coverage profiling (instrumentation overhead distorts timings)")
	}

	// ~15000 exported arrow functions + coordinator to generate ~750KiB
	// of source where V8 parse + compile dominates cold-start.
	var b strings.Builder
	const n = 15000
	for i := 0; i < n; i++ {
		b.WriteString("export const f")
		b.WriteString(itoa(i))
		b.WriteString(" = (x) => x * ")
		b.WriteString(itoa(i + 1))
		b.WriteString(" + '_")
		b.WriteString(itoa(i))
		b.WriteString("';\n")
	}
	b.WriteString("export function sum(x) { let s = ''; ")
	for i := 0; i < 100; i++ {
		b.WriteString("s += f")
		b.WriteString(itoa(i))
		b.WriteString("(x); ")
	}
	b.WriteString("return s; }\n")
	source := b.String()

	const iters = 6

	// Measure fresh ESM eval path
	sourceTotal := time.Duration(0)
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate()
		ctx := v8.NewContext(iso)
		mod, err := ctx.CompileModule(source, "big.mjs")
		if err != nil {
			t.Fatalf("CompileModule: %v", err)
		}
		if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module { return nil }); err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if _, err := mod.Evaluate(); err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		ns := mod.GetNamespace()
		if err := ctx.Global().Set("m", ns); err != nil {
			t.Fatalf("global.Set: %v", err)
		}
		if _, err := ctx.RunScript(`m.sum(1)`, "probe.js"); err != nil {
			t.Fatalf("probe: %v", err)
		}
		mod.Close()
		ctx.Close()
		iso.Dispose()
		sourceTotal += time.Since(start)
	}

	// Build ESM snapshot
	sc := v8.NewSnapshotCreator()
	sctx := sc.Context()
	mod, err := sctx.CompileModule(source, "big.mjs")
	if err != nil {
		t.Fatalf("CompileModule snap: %v", err)
	}
	if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module { return nil }); err != nil {
		t.Fatalf("Instantiate snap: %v", err)
	}
	if _, err := mod.Evaluate(); err != nil {
		t.Fatalf("Evaluate snap: %v", err)
	}
	ns := mod.GetNamespace()
	if err := sctx.Global().Set("m", ns); err != nil {
		t.Fatalf("global.Set: %v", err)
	}
	mod.Close()
	blob, err := sc.CreateBlob(v8.FunctionCodeKeep)
	if err != nil {
		t.Fatalf("CreateBlob: %v", err)
	}
	sc.Dispose()

	// Measure snapshot restore path
	snapTotal := time.Duration(0)
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
		ctx := v8.NewContext(iso)
		if _, err := ctx.RunScript(`m.sum(1)`, "probe.js"); err != nil {
			t.Fatalf("snap probe: %v", err)
		}
		ctx.Close()
		iso.Dispose()
		snapTotal += time.Since(start)
	}

	measure := func() float64 {
		avgSource := sourceTotal / time.Duration(iters)
		avgSnap := snapTotal / time.Duration(iters)
		if avgSnap == 0 {
			t.Fatal("snapshot avg is zero")
		}
		s := float64(avgSource) / float64(avgSnap)
		t.Logf("ESM cold from source = %v, from snapshot = %v, speedup = %.2fx", avgSource, avgSnap, s)
		return s
	}

	const minSpeedup = 2.5
	speedup := measure()
	if speedup >= minSpeedup {
		return
	}

	// CI runners are noisy; retry once to filter scheduling jitter.
	t.Logf("%.2fx < %.1fx threshold, retrying…", speedup, minSpeedup)
	sourceTotal = 0
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate()
		ctx := v8.NewContext(iso)
		mod, err := ctx.CompileModule(source, "big.mjs")
		if err != nil {
			t.Fatalf("retry CompileModule: %v", err)
		}
		if err := mod.Instantiate(func(specifier string, referrerHash int) *v8.Module { return nil }); err != nil {
			t.Fatalf("retry Instantiate: %v", err)
		}
		if _, err := mod.Evaluate(); err != nil {
			t.Fatalf("retry Evaluate: %v", err)
		}
		ns := mod.GetNamespace()
		if err := ctx.Global().Set("m", ns); err != nil {
			t.Fatalf("retry global.Set: %v", err)
		}
		if _, err := ctx.RunScript(`m.sum(1)`, "probe.js"); err != nil {
			t.Fatalf("retry probe: %v", err)
		}
		mod.Close()
		ctx.Close()
		iso.Dispose()
		sourceTotal += time.Since(start)
	}
	snapTotal = 0
	for i := 0; i < iters; i++ {
		start := time.Now()
		iso := v8.NewIsolate(v8.WithSnapshotBlob(blob))
		ctx := v8.NewContext(iso)
		if _, err := ctx.RunScript(`m.sum(1)`, "probe.js"); err != nil {
			t.Fatalf("retry snap probe: %v", err)
		}
		ctx.Close()
		iso.Dispose()
		snapTotal += time.Since(start)
	}

	speedup = measure()
	if speedup < minSpeedup {
		t.Fatalf("ESM snapshot cold-start speedup = %.2fx, want >= %.1fx (failed after retry)", speedup, minSpeedup)
	}
}

// --- PackBundleESM Integration Tests ---

func TestPackBundleESM_HappyPath(t *testing.T) {
	p, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `
			export const greeting = "packed-hello";
			export function render(name) { return greeting + " " + name; }
		`,
		EntryOrigin: "entry.mjs",
		FCH:         v8.FunctionCodeKeep,
		BridgeKey:   "__app",
		Extra:       map[string]string{"build": "test-esm-1"},
	})
	if err != nil {
		t.Fatalf("PackBundleESM: %v", err)
	}
	if p.V8ABI == "" {
		t.Fatal("V8ABI empty")
	}
	if p.RefsDigest == "" {
		t.Fatal("RefsDigest empty")
	}
	if len(p.BundleSHA256) != 64 {
		t.Fatalf("BundleSHA256 length = %d", len(p.BundleSHA256))
	}
	if p.Extra["build"] != "test-esm-1" {
		t.Fatalf("Extra lost: %v", p.Extra)
	}

	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`__app.render("world")`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "packed-hello world" {
		t.Fatalf("render = %q, want %q", got, "packed-hello world")
	}
}

func TestPackBundleESM_WithChunks(t *testing.T) {
	p, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `
			import { helper } from './chunk.mjs';
			export function main(x) { return helper(x) + "!"; }
		`,
		EntryOrigin: "entry.mjs",
		Chunks: map[string]string{
			"./chunk.mjs": `export function helper(x) { return "helped-" + x; }`,
		},
		FCH:       v8.FunctionCodeKeep,
		BridgeKey: "__app",
	})
	if err != nil {
		t.Fatalf("PackBundleESM: %v", err)
	}

	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`__app.main("test")`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "helped-test!" {
		t.Fatalf("main = %q, want %q", got, "helped-test!")
	}
}

func TestPackBundleESM_DefaultBridgeKey(t *testing.T) {
	p, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `export const val = 777;`,
		EntryOrigin: "default-key.mjs",
		FCH:         v8.FunctionCodeKeep,
	})
	if err != nil {
		t.Fatalf("PackBundleESM: %v", err)
	}

	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`__esmExports.val`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.Int32(); got != 777 {
		t.Fatalf("val = %d, want 777", got)
	}
}

func TestPackBundleESM_MarshalUnmarshal(t *testing.T) {
	p, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `export const data = [1, 2, 3];`,
		EntryOrigin: "serial.mjs",
		FCH:         v8.FunctionCodeKeep,
		BridgeKey:   "__s",
		Extra:       map[string]string{"format": "esm"},
	})
	if err != nil {
		t.Fatalf("PackBundleESM: %v", err)
	}

	wire, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := v8.UnmarshalPackedSnapshot(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Extra["format"] != "esm" {
		t.Fatalf("Extra lost: %v", got.Extra)
	}

	iso, err := got.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`JSON.stringify(__s.data)`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "[1,2,3]" {
		t.Fatalf("data = %q, want %q", got, "[1,2,3]")
	}
}

func TestPackBundleESM_EmptySource(t *testing.T) {
	_, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: "",
		EntryOrigin: "empty.mjs",
	})
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestPackBundleESM_ChunkCompileError(t *testing.T) {
	_, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `import { x } from './bad.mjs'; export const y = x;`,
		EntryOrigin: "entry.mjs",
		Chunks: map[string]string{
			"./bad.mjs": `export {`, // syntax error
		},
		FCH: v8.FunctionCodeKeep,
	})
	if err == nil {
		t.Fatal("expected error for chunk compile failure")
	}
}

func TestPackBundleESM_EntryCompileError(t *testing.T) {
	_, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `export {`, // syntax error
		EntryOrigin: "bad-entry.mjs",
		FCH:         v8.FunctionCodeKeep,
	})
	if err == nil {
		t.Fatal("expected error for entry compile failure")
	}
}

func TestPackBundleESM_EntryInstantiateError(t *testing.T) {
	_, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource: `import { x } from './missing.mjs'; export const y = x;`,
		EntryOrigin: "entry.mjs",
		FCH:         v8.FunctionCodeKeep,
	})
	if err == nil {
		t.Fatal("expected error for entry instantiate failure (missing dep)")
	}
}

func TestPackBundleESM_ExistingBlob(t *testing.T) {
	// Build a base blob with a polyfill
	basePack, err := v8.PackBundle(v8.PackOptions{
		Source: `globalThis.polyfill = (x) => x.toUpperCase();`,
		Origin: "polyfill.js",
		FCH:    v8.FunctionCodeKeep,
	})
	if err != nil {
		t.Fatalf("PackBundle base: %v", err)
	}

	// Stack ESM on top
	p, err := v8.PackBundleESM(v8.PackESMOptions{
		EntrySource:  `export function upper(s) { return polyfill(s); }`,
		EntryOrigin:  "app.mjs",
		FCH:          v8.FunctionCodeKeep,
		ExistingBlob: basePack.Blob,
		BridgeKey:    "__app",
	})
	if err != nil {
		t.Fatalf("PackBundleESM: %v", err)
	}

	iso, err := p.RestoreIsolate(v8.RestoreOptions{})
	if err != nil {
		t.Fatalf("RestoreIsolate: %v", err)
	}
	defer iso.Dispose()
	c := v8.NewContext(iso)
	defer c.Close()

	val, err := c.RunScript(`__app.upper("hello")`, "probe.js")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got := val.String(); got != "HELLO" {
		t.Fatalf("upper = %q, want %q", got, "HELLO")
	}
}
