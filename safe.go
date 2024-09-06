//go:build !unsafe

package main

// stub to not inject unsafe primitives
func optionalUnsafe(env *Obj) *Obj { return env }
