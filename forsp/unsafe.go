//go:build unsafe

package forsp

import "unsafe"

// Low-level primitives
func (f *Forsp) primPtrState(_ **Obj) {
	p := uintptr(unsafe.Pointer(f))
	f.Push(NewNumber(int64(p)))
}

func (f *Forsp) primPtrRead(_ **Obj) {
	n := (*int64)(unsafe.Pointer(f.Pop()))
	f.Push(NewNumber(*n))
}

func (f *Forsp) primPtrWrite(_ **Obj) {
	b, a := f.Pop(), (*int64)(unsafe.Pointer(f.Pop()))
	*a = int64(*b.Number)
}

func (f *Forsp) primPtrToObj(_ **Obj) {
	p := unsafe.Pointer(f.Pop())
	f.Push((*Obj)(p))
}

func (f *Forsp) primPtrFromObj(_ **Obj) {
	a := f.Pop()
	p := unsafe.Pointer(a)
	f.Push(NewNumber(int64(uintptr(p))))
}

func optionalUnsafe(f *Forsp, env *Obj) *Obj {
	// Low-level primitives
	env = f.EnvDefinePrim(env, "ptr-state!", f.primPtrState)
	env = f.EnvDefinePrim(env, "ptr-read!", f.primPtrRead)
	env = f.EnvDefinePrim(env, "ptr-write!", f.primPtrWrite)
	env = f.EnvDefinePrim(env, "ptr-to-obj!", f.primPtrToObj)
	env = f.EnvDefinePrim(env, "ptr-from-obj!", f.primPtrFromObj)

	return env
}
