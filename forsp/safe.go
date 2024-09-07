//go:build !unsafe

package forsp

// stub to not inject unsafe primitives
func optionalUnsafe(_ *Forsp, env *Obj) *Obj { return env }
