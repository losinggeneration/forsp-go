package forsp

import (
	"fmt"
	"io"
	"strconv"
)

type Tag uint

const (
	TagNil       Tag = iota
	TagAtom      Tag = iota
	TagNumber    Tag = iota
	TagPair      Tag = iota
	TagClosure   Tag = iota
	TagPrimitive Tag = iota
)

type (
	Nil    struct{}
	Atom   string
	Number int64
	Pair   struct {
		car, cdr *Obj
	}
	Closure struct {
		body, env *Obj
	}
	Primitive func(env **Obj)
)

type Obj struct {
	*Nil
	*Atom
	*Number
	*Pair
	*Closure
	*Primitive
	Tag Tag
}

func (o Obj) String() string {
	switch o.Tag {
	case TagNil:
		return "Nil"
	case TagAtom:
		return string(*o.Atom)
	case TagNumber:
		return strconv.FormatInt(int64(*o.Number), 10)
	case TagPair:
		return "Pair"
	case TagClosure:
		return "Closure"
	case TagPrimitive:
		return "Primitive"
	}

	return "unknown"
}

type Forsp struct {
	nil       *Obj // nil: ()
	readStack *Obj // defered obj to emit from read

	// Atoms
	internedAtoms *Obj // interned atoms list
	AtomTrue      *Obj // atom: t
	AtomQuote     *Obj // atom: quote
	AtomPush      *Obj // atom: push
	AtomPop       *Obj // atom: pop

	// Stack/env
	Stack *Obj // top-of-stack (implemented with pairs)
	Env   *Obj // top-level / initial environment

	input string // input data string used by read()
	pos   uint64 // input data position used by read()
}

func NewNil() *Obj {
	return &Obj{Tag: TagNil, Nil: &Nil{}}
}

func NewAtom(str string) *Obj {
	return &Obj{Tag: TagAtom, Atom: ptr(Atom(str))}
}

func NewNumber(n int64) *Obj {
	return &Obj{Tag: TagNumber, Number: ptr(Number(n))}
}

func NewPair(car, cdr *Obj) *Obj {
	return &Obj{Tag: TagPair, Pair: &Pair{car: car, cdr: cdr}}
}

func NewClosure(body, env *Obj) *Obj {
	return &Obj{Tag: TagClosure, Closure: &Closure{body: body, env: env}}
}

func NewPrimitive(f func(env **Obj)) *Obj {
	return &Obj{Tag: TagPrimitive, Primitive: ptr(Primitive(f))}
}

func assert(v bool, msg string) {
	if !v {
		panic(fmt.Sprintf("ASSERT: %s", msg))
	}
}

func assertTag(v *Obj, t Tag, msg string) {
	assert(is(v, t), msg)
}

func failTag(v *Obj, t Tag, msg string) {
	if !is(v, t) {
		fail(msg)
	}
}

func fail(msg string) {
	panic(fmt.Sprintf("FAIL: %s", msg))
}

func failf(msg string, args ...any) {
	fail(fmt.Sprintf(msg, args...))
}

func is(v *Obj, t Tag) bool {
	return v.Tag == t
}

func ptr[T any](v T) *T {
	return &v
}

func (f *Forsp) intern(atomBuf string) *Obj {
	for list := f.internedAtoms; list != f.nil; list = list.cdr {
		assertTag(list, TagPair, fmt.Sprintf("interned_atoms must be Pairs got %v", list))

		elem := list.car
		assertTag(elem, TagAtom, fmt.Sprintf("interned_atoms.car must be an Atom got %v", elem))
		if len(atomBuf) == len(*elem.Atom) && atomBuf == string(*elem.Atom) {
			return elem
		}
	}

	// not found, create a new one and push the front of the list
	atom := NewAtom(atomBuf)
	f.internedAtoms = NewPair(atom, f.internedAtoms)

	return atom
}

func car(obj *Obj) *Obj {
	failTag(obj, TagPair, fmt.Sprintf("Expected Pair to apply car() function got %v", obj))
	return obj.car
}

func cdr(obj *Obj) *Obj {
	failTag(obj, TagPair, fmt.Sprintf("Expected Pair to apply cdr() function got %v", obj))
	return obj.cdr
}

func ObjEqual(a *Obj, b *Obj) bool {
	return *a == *b || (is(a, TagNumber) && is(b, TagNumber) && *a.Number == *b.Number)
}

func ObjToInt64(a *Obj) int64 {
	if is(a, TagNumber) {
		return int64(*a.Number)
	}

	return 0
}

func (f *Forsp) peek() byte {
	if f.pos == uint64(len(f.input)) {
		return 0
	}

	return f.input[f.pos]
}

func (f *Forsp) advance() {
	assert(f.peek() != 0, "cannot advance further")
	f.pos++
}

func isWhite(c uint8) bool { return c == ' ' || c == '\t' || c == '\n' }

func isDirective(c uint8) bool { return c == '\'' || c == '^' || c == '$' }

func isPunctuation(c uint8) bool {
	return c == 0 || isWhite(c) || isDirective(c) || c == '(' || c == ')' || c == ';'
}

func (f *Forsp) skipWhiteAndComments() {
	c := f.peek()
	if c == 0 {
		return
	}

	// skip whitespace
	if isWhite(c) {
		f.advance()
		f.skipWhiteAndComments()
		return
	}

	// skip comment
	if c == ';' {
		f.advance()
		for {
			c = f.peek()
			if c == 0 {
				return
			}
			f.advance()
			if c == '\n' {
				break
			}
		}

		f.skipWhiteAndComments()
		return
	}
}

func (f *Forsp) readList() *Obj {
	if f.readStack == nil {
		f.skipWhiteAndComments()
		c := f.peek()
		if c == ')' {
			f.advance()
			return f.nil
		}
	}

	first := f.Read()
	second := f.readList()
	return NewPair(first, second)
}

func (f *Forsp) parseInt64(str string) (int64, bool) {
	i, err := strconv.ParseInt(str, 10, 64)
	return i, err == nil
}

func (f *Forsp) readScalar() *Obj {
	// otherwise, assume atom or number and read it
	start := f.pos
	for !isPunctuation(f.peek()) {
		f.advance()
	}

	str := f.input[start:f.pos]
	// is it a number?
	if n, ok := f.parseInt64(str); ok {
		return NewNumber(n)
	}

	// atom
	return f.intern(str)
}

func (f *Forsp) Read() *Obj {
	read_stack := f.readStack
	if read_stack != nil {
		f.readStack = cdr(read_stack)
		return car(read_stack)
	}

	f.skipWhiteAndComments()

	c := f.peek()
	switch c {
	case 0:
		fail("End of input: could not read()")

	// A quote?
	case '\'':
		f.advance()
		return f.AtomQuote

	// A push?
	case '^':
		f.advance()
		var s *Obj
		s = NewPair(f.AtomPush, s)
		s = NewPair(f.readScalar(), s)
		s = NewPair(f.AtomQuote, s)
		f.readStack = s

		return f.Read()

	// A pop?
	case '$':
		f.advance()
		var s *Obj
		s = NewPair(f.AtomPop, s)
		s = NewPair(f.readScalar(), s)
		s = NewPair(f.AtomQuote, s)
		f.readStack = s

		return f.Read()

	// Read a list?
	case '(':
		f.advance()
		return f.readList()

	}

	return f.readScalar()
}

func (f *Forsp) printListTail(obj *Obj) {
	if obj == f.nil {
		fmt.Print(")")
		return
	}

	if is(obj, TagPair) {
		fmt.Print(" ")
		f.PrintRecurse(obj.car)
		f.printListTail(obj.cdr)
	} else {
		fmt.Print(" . ")
		f.PrintRecurse(obj)
		fmt.Print(")")
	}
}

func (f *Forsp) PrintRecurse(obj *Obj) {
	if obj == f.nil {
		fmt.Print("()")
		return
	}

	switch obj.Tag {
	case TagNil: // do nothing
	case TagAtom:
		fmt.Print(*obj.Atom)
	case TagNumber:
		fmt.Print(*obj.Number)
	case TagPair:
		fmt.Print("(")
		f.PrintRecurse(obj.car)
		f.printListTail(obj.cdr)

	case TagClosure:
		fmt.Print("CLOSURE<")
		f.PrintRecurse(obj.body)
		fmt.Printf(", %p>", obj.env)

	case TagPrimitive:
		fmt.Printf("PRIM<%p>", obj.Primitive)
	}
}

func (f *Forsp) Print(obj *Obj) {
	f.PrintRecurse(obj)
	fmt.Println()
}

func (f *Forsp) EnvFind(env *Obj, key *Obj) *Obj {
	if !is(key, TagAtom) {
		failf("Expected 'key' to be an Atom in env_find() got %v", key)
	}

	for v := env; v != f.nil; v = cdr(v) {
		kv := car(v)
		if key == car(kv) || *key == *car(kv) {
			return cdr(kv)
		}
	}

	failf("Failed to find key='%s' in environment", *key.Atom)
	return nil
}

func (f *Forsp) EnvDefine(env *Obj, key *Obj, val *Obj) *Obj {
	return NewPair(NewPair(key, val), env)
}

func (f *Forsp) EnvDefinePrim(env *Obj, name string, fn func(env **Obj)) *Obj {
	return f.EnvDefine(env, f.intern(name), NewPrimitive(fn))
}

func (f *Forsp) Push(obj *Obj) {
	f.Stack = NewPair(obj, f.Stack)
}

func (f *Forsp) tryPop() (*Obj, bool) {
	if f.Stack == f.nil {
		return nil, false
	}

	o := car(f.Stack)
	f.Stack = cdr(f.Stack)
	return o, true
}

func (f *Forsp) Pop() *Obj {
	if ret, ok := f.tryPop(); ok {
		return ret
	}

	fail("Value Stack Underflow")
	return nil
}

func (f *Forsp) ComputeEnv(comp *Obj, env *Obj) {
	for comp != f.nil {
		cmd := car(comp)
		comp = cdr(comp)

		if cmd == f.AtomQuote {
			if comp == f.nil {
				fail("Expected data following a quote form")
			}
			f.Push(car(comp))
			comp = cdr(comp)

			continue
		}

		f.eval(cmd, &env)
	}
}

func (f *Forsp) eval(expr *Obj, env **Obj) {
	if is(expr, TagAtom) {
		val := f.EnvFind(*env, expr)
		if is(val, TagClosure) {
			f.ComputeEnv(val.body, val.env)
		} else if is(val, TagPrimitive) {
			(*val.Primitive)(env)
		} else {
			f.Push(val)
		}
	} else if is(expr, TagNil) || is(expr, TagPair) {
		f.Push(NewClosure(expr, *env))
	} else {
		f.Push(expr)
	}
}

// Core primitives
func (f *Forsp) primPush(env **Obj) { f.Push(f.EnvFind(*env, f.Pop())) }

func (f *Forsp) primPop(env **Obj) {
	k, v := f.Pop(), f.Pop()
	*env = f.EnvDefine(*env, k, v)
}

func (f *Forsp) primEq(_ **Obj) {
	if ObjEqual(f.Pop(), f.Pop()) {
		f.Push(f.AtomTrue)
	} else {
		f.Push(f.nil)
	}
}

func (f *Forsp) primCons(_ **Obj) {
	a, b := f.Pop(), f.Pop()
	f.Push(NewPair(a, b))
}

func (f *Forsp) primCar(_ **Obj) { f.Push(car(f.Pop())) }
func (f *Forsp) primCdr(_ **Obj) { f.Push(cdr(f.Pop())) }

func (f *Forsp) primCswap(_ **Obj) {
	if f.Pop() == f.AtomTrue {
		a, b := f.Pop(), f.Pop()
		f.Push(a)
		f.Push(b)
	}
}

func (f *Forsp) primTag(_ **Obj)   { f.Push(NewNumber(int64(f.Pop().Tag))) }
func (f *Forsp) primRead(_ **Obj)  { f.Push(f.Read()) }
func (f *Forsp) primPrint(_ **Obj) { f.Print(f.Pop()) }

// Extra primitives
func (f *Forsp) primStack(_ **Obj) { f.Push(f.Stack) }
func (f *Forsp) primEnv(env **Obj) { f.Push(*env) }

func (f *Forsp) primSub(_ **Obj) {
	b, a := f.Pop(), f.Pop()
	f.Push(NewNumber(ObjToInt64(a) - ObjToInt64(b)))
}

func (f *Forsp) primMul(_ **Obj) {
	b, a := f.Pop(), f.Pop()
	f.Push(NewNumber(ObjToInt64(a) * ObjToInt64(b)))
}

func (f *Forsp) primNand(_ **Obj) {
	b, a := f.Pop(), f.Pop()
	f.Push(NewNumber(^(ObjToInt64(a) & ObjToInt64(b))))
}

func (f *Forsp) primLsh(_ **Obj) {
	b, a := f.Pop(), f.Pop()
	f.Push(NewNumber(ObjToInt64(a) << uint(ObjToInt64(b))))
}

func (f *Forsp) primRsh(_ **Obj) {
	b, a := f.Pop(), f.Pop()
	f.Push(NewNumber(ObjToInt64(a) >> uint(ObjToInt64(b))))
}

func New() *Forsp {
	f := Forsp{}

	f.nil = NewNil()

	f.internedAtoms = f.nil
	f.AtomTrue = f.intern("t")
	f.AtomQuote = f.intern("quote")
	f.AtomPush = f.intern("push")
	f.AtomPop = f.intern("pop")

	f.Stack = f.nil

	env := f.nil

	// core primitives
	env = f.EnvDefinePrim(env, "push", f.primPush)
	env = f.EnvDefinePrim(env, "pop", f.primPop)
	env = f.EnvDefinePrim(env, "cons", f.primCons)
	env = f.EnvDefinePrim(env, "car", f.primCar)
	env = f.EnvDefinePrim(env, "cdr", f.primCdr)
	env = f.EnvDefinePrim(env, "eq", f.primEq)
	env = f.EnvDefinePrim(env, "cswap", f.primCswap)
	env = f.EnvDefinePrim(env, "tag", f.primTag)
	env = f.EnvDefinePrim(env, "read", f.primRead)
	env = f.EnvDefinePrim(env, "print", f.primPrint)

	// extra primitives
	env = f.EnvDefinePrim(env, "stack", f.primStack)
	env = f.EnvDefinePrim(env, "env", f.primEnv)
	env = f.EnvDefinePrim(env, "-", f.primSub)
	env = f.EnvDefinePrim(env, "*", f.primMul)
	env = f.EnvDefinePrim(env, "nand", f.primNand)
	env = f.EnvDefinePrim(env, "<<", f.primLsh)
	env = f.EnvDefinePrim(env, ">>", f.primRsh)

	env = optionalUnsafe(&f, env)

	f.Env = env

	return &f
}

func (f *Forsp) SetReader(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	f.input = string(b)
	f.pos = 0

	return nil
}

func (f *Forsp) Compute(obj *Obj) {
	f.ComputeEnv(obj, f.Env)
}
