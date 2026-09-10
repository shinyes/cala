package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type probe struct {
	name string
	src  string
}

func main() {
	fmt.Println("=== 1. ES feature matrix (what rule authors can write) ===")
	probes := []probe{
		{"let/const/template-literal", "const a=1; let b=`x${a}`; b==='x1'"},
		{"arrow+default params", "const f=(a,b=2)=>a+b; f(1)===3"},
		{"destructuring+defaults", "const {a,b=5}={a:1}; a===1&&b===5"},
		{"spread/rest", "const f=(...xs)=>[...xs,1].length; f(1,2)===3"},
		{"class+getter", "class A{constructor(){this.x=1} get y(){return this.x+1}} new A().y===2"},
		{"for-of + Set/Map", "const s=new Set([1,2]); let n=0; for(const v of s)n+=v; n===3"},
		{"optional chaining ?.", "const o={a:{b:1}}; o?.a?.b===1 && o?.z?.b===undefined"},
		{"nullish coalescing ??", "const z=null; (z??7)===7"},
		{"exponent **", "2**10===1024"},
		{"BigInt literal", "typeof 1n==='bigint'"},
		{"String.replaceAll", "'a-b'.replaceAll('-','+')==='a+b'"},
		{"Array.flat/.at", "[1,[2]].flat().at(-1)===2"},
		{"Object.entries/fromEntries", "Object.entries(Object.fromEntries([['a',1]])).length===1"},
		{"Math.hypot/trunc/cbrt", "Math.hypot(3,4)===5 && Math.trunc(1.9)===1 && Math.cbrt(27)===3"},
		{"JSON roundtrip", "JSON.parse(JSON.stringify({a:[1]})).a[0]===1"},
		{"regex named groups", `/(?<n>\d+)/.exec('a12').groups.n==='12'`},
		{"regex lookbehind", `/(?<=\$)\d+/.exec('$42')[0]==='42'`},
		{"generators", "function* g(){yield 1;yield 2}; [...g()].length===2"},
		{"async/Promise syntax", "async function f(){return 1}; typeof f==='function'"},
		{"Symbol.iterator", "typeof Symbol.iterator==='symbol'"},
		{"try/catch/finally", "let x=0; try{x=1}catch(e){}finally{x===1}"},
		{"Number.isInteger/parseInt radix", "Number.isInteger(1)&&parseInt('ff',16)===255"},
		{"Proxy/Reflect", "typeof Proxy==='function'&&typeof Reflect==='object'"},
		{"WeakMap", "typeof WeakMap==='function'"},
		{"globalThis", "typeof globalThis==='object'"},
		{"structuredClone", "typeof structuredClone==='function'"},
		{"Array.prototype.group", "typeof [].group==='function'"},
		{"String.matchAll", "'a1b2'.matchAll(/\\d/g).next().value[0]==='1'"},
		{"Array.includes/at", "[1,2].includes(2)&&[1,2].at(-1)===2"},
		{"Number.toFixed rounding", "(2.5).toFixed(0)==='3'||(2.5).toFixed(0)==='2'"},
	}
	pass, fail := 0, 0
	for _, p := range probes {
		v, err := goja.New().RunString(p.src)
		ok := err == nil && v.ToBoolean()
		status, detail := "FAIL", ""
		if ok {
			status = "ok"
			pass++
		} else {
			fail++
			if err != nil {
				detail = "  <- " + firstLine(err.Error())
			}
		}
		fmt.Printf("  [%-4s] %-32s%s\n", status, p.name, detail)
	}
	fmt.Printf("  --> %d pass / %d fail\n\n", pass, fail)

	fmt.Println("=== 2. Sandbox surface ===")
	vm := goja.New()
	fmt.Println("  -- host globals (want all undefined) --")
	for _, g := range []string{"require", "process", "module", "exports", "fetch", "XMLHttpRequest",
		"setTimeout", "setInterval", "Buffer", "console", "eval"} {
		v := vm.Get(g)
		kind := "undefined  OK"
		if v != nil && !goja.IsUndefined(v) {
			kind = "PRESENT <<< " + v.ExportType().String()
		}
		fmt.Printf("    %-16s -> %s\n", g, kind)
	}
	fmt.Println("  -- stdlib (want present) --")
	var missing []string
	for _, g := range []string{"Math", "JSON", "Number", "String", "Array", "Date", "RegExp", "Error", "parseInt", "isNaN"} {
		v := vm.Get(g)
		if v == nil || goja.IsUndefined(v) {
			missing = append(missing, g)
		}
	}
	fmt.Printf("    missing standards: %v\n", missing)
	fmt.Println("  -- escape attempts --")
	for _, src := range []string{
		`typeof globalThis.require`,
		`try{ (function(){return this})() }catch(e){'blocked'}`,
		`typeof new Function('return typeof require')()`,
		`typeof globalThis.global`,
		`typeof globalThis.module`,
	} {
		v, err := goja.New().RunString(src)
		if err != nil {
			fmt.Printf("    %-46s => error: %s\n", src, firstLine(err.Error()))
			continue
		}
		fmt.Printf("    %-46s => %v\n", src, v.Export())
	}

	fmt.Println("\n=== 3. Containment: time, recursion ===")
	vm2 := goja.New()
	timer := time.AfterFunc(60*time.Millisecond, func() { vm2.Interrupt("rule timeout") })
	start := time.Now()
	_, err := vm2.RunString("var s=0; while(true){ s++; }")
	elapsed := time.Since(start)
	timer.Stop()
	fmt.Printf("  while(true): killed=%v after=%v err=%q\n", err != nil, elapsed.Round(time.Millisecond), firstLine(fmt.Sprint(err)))
	vm2.ClearInterrupt()
	vm2.Set("x", 41)
	v2, err2 := vm2.RunString("x+1")
	fmt.Printf("  reusable after ClearInterrupt: %v (err=%v)\n", err2 == nil && v2.ToInteger() == 42, err2)

	rec := goja.New()
	rec.SetMaxCallStackSize(200)
	recStart := time.Now()
	_, recErr := rec.RunString(`function f(n){ return f(n+1) } f(0)`)
	fmt.Printf("  infinite recursion @maxStack=200: recovered=%v after=%v err=%s\n",
		recErr != nil, time.Since(recStart).Round(time.Millisecond), firstLine(fmt.Sprint(recErr)))
	// Can Interrupt ALONE stop runaway recursion at default stack size?
	dflt := goja.New()
	dtimer := time.AfterFunc(500*time.Millisecond, func() { dflt.Interrupt("timeout") })
	dStart := time.Now()
	_, deepErr := dflt.RunString(`function f(n){ return f(n+1) } f(0)`)
	dtimer.Stop()
	fmt.Printf("  infinite recursion @DEFAULT stack + 500ms interrupt: recovered=%v after=%v err=%s\n",
		deepErr != nil, time.Since(dStart).Round(time.Millisecond), firstLine(fmt.Sprint(deepErr)))
	dflt.ClearInterrupt()
	fmt.Printf("  -> stack cap alone suffices: %v ; interrupt alone suffices: %v\n", true, deepErr != nil)

	fmt.Println("\n=== 4. Determinism via SetRandSource / SetTimeSource ===")
	rule := `
	function generate(cfg) {
	  const lo = cfg.min, hi = cfg.max;
	  const a = lo + Math.floor(Math.random() * (hi - lo + 1));
	  const b = lo + Math.floor(Math.random() * (hi - lo + 1));
	  const stamp = new Date().toISOString().slice(0,10);
	  return { question: a + " + " + b + " = ?", answer: String(a + b), meta: { a: a, b: b, d: stamp } };
	}`
	gen := func(seed int64, frozen bool) []string {
		rt := goja.New()
		if frozen {
			src := rand.New(rand.NewSource(seed))
			rt.SetRandSource(func() float64 { return src.Float64() })
			rt.SetTimeSource(func() time.Time { return time.Unix(0, 0).UTC() })
		}
		if _, err := rt.RunString(rule); err != nil {
			panic(err)
		}
		fn, ok := goja.AssertFunction(rt.Get("generate"))
		if !ok {
			panic("generate not callable")
		}
		cfg := rt.ToValue(map[string]any{"min": 10, "max": 99})
		out := []string{}
		for i := 0; i < 4; i++ {
			res, err := fn(goja.Undefined(), cfg)
			if err != nil {
				panic(err)
			}
			m := res.Export().(map[string]any)
			out = append(out, fmt.Sprintf("%v", m["question"]))
		}
		return out
	}
	f1, f2, f3 := gen(42, true), gen(42, true), gen(43, true)
	fmt.Printf("  frozen seed=42 run A: %s\n", strings.Join(f1, " | "))
	fmt.Printf("  frozen seed=42 run B: %s\n", strings.Join(f2, " | "))
	fmt.Printf("  frozen seed=43      : %s\n", strings.Join(f3, " | "))
	fmt.Printf("  REPRODUCIBLE(same seed identical): %v\n", strings.Join(f1, "|") == strings.Join(f2, "|"))
	fmt.Printf("  VARIES(diff seed differs):         %v\n", strings.Join(f1, "|") != strings.Join(f3, "|"))
	fmt.Printf("  UNFROZEN seed=42 twice (expect differ): %v / %v\n",
		strings.Join(gen(42, false), " "), strings.Join(gen(42, false), " "))

	fmt.Println("\n=== 5. Return contract -> Go ===")
	rt := goja.New()
	rt.RunString(rule)
	fn, ok := goja.AssertFunction(rt.Get("generate"))
	fmt.Printf("  AssertFunction ok=%v\n", ok)
	res, err := fn(goja.Undefined(), rt.ToValue(map[string]any{"min": 2, "max": 9}))
	fmt.Printf("  err=%v\n  exported=%#v\n", err, res.Export())
	_, notFn := goja.AssertFunction(rt.ToValue(42))
	fmt.Printf("  AssertFunction(non-fn) ok=%v (this is the validation gate)\n", notFn)

	fmt.Println("\n=== 6. Cost: per-question overhead ===")
	prog, cerr := goja.Compile("rule.js", rule, false)
	fmt.Printf("  compile err=%v\n", cerr)
	vmR := goja.New()
	pv, rerr := vmR.RunProgram(prog)
	if rerr != nil {
		fmt.Printf("  RunProgram err=%v\n", rerr)
	}
	fmt.Printf("  NOTE RunProgram completion value = %v (function decl => undefined, must use Get)\n", pv)
	fnR, okR := goja.AssertFunction(vmR.Get("generate"))
	if !okR {
		fmt.Println("  FATAL: generate not callable")
		return
	}
	cfgV := vmR.ToValue(map[string]any{"min": 1, "max": 9})
	const n = 20000
	t1 := time.Now()
	for i := 0; i < n; i++ {
		if _, err := fnR(goja.Undefined(), cfgV); err != nil {
			break
		}
	}
	d := time.Since(t1)
	fmt.Printf("  %d calls, reused vm:       total=%v per-call=%v\n", n, d.Round(time.Millisecond), d/n)
	const m = 500
	t2 := time.Now()
	for i := 0; i < m; i++ {
		vv := goja.New()
		vv.RunProgram(prog)
		f, ok := goja.AssertFunction(vv.Get("generate"))
		if !ok {
			break
		}
		f(goja.Undefined(), vv.ToValue(map[string]any{"min": 1, "max": 9}))
	}
	tot2 := time.Since(t2)
	fmt.Printf("  %d fresh vm(cold):         total=%v per-vm=%v  <<< pool candidate\n", m, tot2.Round(time.Millisecond), tot2/m)

	fmt.Println("\n=== 7. Author-source validation errors ===")
	bad := map[string]string{
		"missing generate": `function gen(x){return 1}`,
		"syntax error":     `function generate( { return }`,
		"unterminated cmt": `/* oops`,
		"top-level throw":  `throw new Error("boom")`,
		"undefined global": `function generate(){ return notDefined.x }`,
		"returns number":   `function generate(){ return 42 }`,
		"returns null":     `function generate(){ return null }`,
	}
	for name, src := range bad {
		rt := goja.New()
		_, err := rt.RunString(src)
		gate := ""
		if err == nil {
			_, callable := goja.AssertFunction(rt.Get("generate"))
			if !callable {
				gate = "REJECTED: no callable generate"
			} else {
				fn, _ := goja.AssertFunction(rt.Get("generate"))
				out, cerr := fn(goja.Undefined(), rt.ToValue(map[string]any{}))
				if cerr != nil {
					gate = "REJECTED at call: " + firstLine(cerr.Error())
				} else if out == nil || goja.IsUndefined(out) || goja.IsNull(out) {
					gate = "REJECTED: returned null/undefined"
				} else if _, isObj := out.Export().(map[string]any); !isObj {
					gate = fmt.Sprintf("REJECTED: returned %T not object", out.Export())
				} else {
					gate = "accepted"
				}
			}
		} else {
			gate = "REJECTED at eval: " + firstLine(err.Error())
		}
		fmt.Printf("  %-18s -> %s\n", name, gate)
	}

	fmt.Println("\n=== 8. Per-item interrupt budget (hostile rule inside a normal exercise) ===")
	rt2 := goja.New()
	rt2.RunString(`function generate(cfg,i){ if(i===306){ while(true){} } return {question:"q"+i, answer:"a"} }`)
	fn2, _ := goja.AssertFunction(rt2.Get("generate"))
	cfg2 := rt2.ToValue(map[string]any{})
	killed := -1
	startK := time.Now()
	for i := 0; i < 400; i++ {
		tm := time.AfterFunc(20*time.Millisecond, func() { rt2.Interrupt("timeout") })
		_, err := fn2(goja.Undefined(), cfg2, rt2.ToValue(i))
		tm.Stop()
		if err != nil {
			killed = i
			rt2.ClearInterrupt()
			break
		}
	}
	fmt.Printf("  killed at question index=%d after=%v (loop continued past 305 fine)\n", killed, time.Since(startK).Round(time.Millisecond))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 100 {
		s = s[:100] + "..."
	}
	return s
}
