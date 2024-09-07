//go:build unsafe

package forsp

import "unsafe"

// Low-level primitives
func (f *Forsp) primPtrState(_ **Obj) {
	p := uintptr(unsafe.Pointer(&state))
	push(numberNew(int64(p)))
}

func primPtrRead(_ **Obj) {
	n := (*int64)(unsafe.Pointer(uintptr(objInt64(pop()))))
	push(numberNew(*n))
}

func primPtrWrite(_ **Obj) {
	b, a := pop(), (*int64)(unsafe.Pointer(uintptr(objInt64(pop()))))
	*a = objInt64(b)
}

func primPtrToObj(_ **Obj) {
	p := unsafe.Pointer(uintptr(objInt64(pop())))
	push((*Obj)(p))
}

func primPtrFromObj(_ **Obj) {
	a := pop()
	p := unsafe.Pointer(a)
	push(numberNew(int64(uintptr(p))))
}

func optionalUnsafe(f *Forsp, env *Obj) *Obj {
	// Low-level primitives
	env = envDefinePrim(env, "ptr-state!", f.primPtrState)
	env = envDefinePrim(env, "ptr-read!", primPtrRead)
	env = envDefinePrim(env, "ptr-write!", primPtrWrite)
	env = envDefinePrim(env, "ptr-to-obj!", primPtrToObj)
	env = envDefinePrim(env, "ptr-from-obj!", primPtrFromObj)

	return env
}
