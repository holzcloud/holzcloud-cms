// Ein Plugin, das gerade genug tut, um die Aufrufkonvention zu beweisen:
// es hallt wider, es schreibt und liest im eigenen Speicher, es greift nach
// einer Berechtigung, die es nicht hat, und es hängt sich auf Befehl auf.
package main

import (
	"encoding/json"
	"unsafe"
)

//go:wasmimport holzcloud hc_call
func hcCall(opPtr, opLen, argPtr, argLen, outPtr, outCap uint32) uint64

// puffer bleibt am Leben, solange der Host liest. Ohne die Referenz könnte der
// Sammler ihn einziehen, während der Host noch darin liest.
var puffer []byte

//go:wasmexport hc_alloc
func alloc(n int32) int32 {
	puffer = make([]byte, n)
	return int32(uintptr(unsafe.Pointer(unsafe.SliceData(puffer))))
}

// Haken und Nutzlast kommen in einem Puffer, Haken zuerst.
func teile(hookLen, totalLen int32) (string, []byte) {
	if hookLen < 0 || totalLen < hookLen || int(totalLen) > len(puffer) {
		return "", nil
	}
	return string(puffer[:hookLen]), puffer[hookLen:totalLen]
}

var antwort []byte

func ruf(op string, arg any) (uint32, []byte) {
	a, _ := json.Marshal(arg)
	out := make([]byte, 4096)
	for {
		ob := unsafe.SliceData(out)
		r := hcCall(
			uint32(uintptr(unsafe.Pointer(unsafe.StringData(op)))), uint32(len(op)),
			uint32(uintptr(unsafe.Pointer(unsafe.SliceData(a)))), uint32(len(a)),
			uint32(uintptr(unsafe.Pointer(ob))), uint32(len(out)))
		status, n := uint32(r>>32), uint32(r)
		if status == 2 { // zu klein: mit der genannten Grösse noch einmal
			out = make([]byte, n)
			continue
		}
		return status, out[:n]
	}
}

//go:wasmexport hc_handle
func handle(ptr, hookLen, totalLen int32) uint64 {
	hook, in := teile(hookLen, totalLen)

	var res any
	switch hook {
	case "content":
		var c struct {
			HTML string `json:"html"`
		}
		json.Unmarshal(in, &c)
		res = map[string]any{"html": c.HTML + "<!-- echo -->", "changed": true}

	case "event":
		var e struct {
			Name string            `json:"name"`
			Data map[string]string `json:"data"`
		}
		json.Unmarshal(in, &e)
		switch e.Data["tue"] {
		case "schreiben":
			ruf("store.set", map[string]any{"key": e.Data["key"], "value": e.Data["value"]})
		case "lesen":
			_, b := ruf("store.get", map[string]any{"key": e.Data["key"]})
			res = map[string]any{"gelesen": string(b)}
		case "verboten":
			st, b := ruf("settings", map[string]any{})
			res = map[string]any{"status": st, "meldung": string(b)}
		case "protokoll":
			ruf("log", map[string]any{"level": "warn", "message": e.Data["value"]})
		case "unbekannt":
			st, b := ruf("gibtsnicht", map[string]any{})
			res = map[string]any{"status": st, "meldung": string(b)}
		case "endlos":
			for {
			}
		case "grosse-antwort":
			_, b := ruf("store.get", map[string]any{"key": e.Data["key"]})
			res = map[string]any{"laenge": len(b)}
		case "muell":
			antwort = []byte("{das ist kein json")
			return uint64(uintptr(unsafe.Pointer(unsafe.SliceData(antwort))))<<32 | uint64(len(antwort))
		}

	case "request":
		res = map[string]any{"handled": false}
	}

	if res == nil {
		return 0
	}
	antwort, _ = json.Marshal(res)
	return uint64(uintptr(unsafe.Pointer(unsafe.SliceData(antwort))))<<32 | uint64(len(antwort))
}

func main() {}
