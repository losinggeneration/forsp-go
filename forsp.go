package main

import (
	"fmt"
	"io"
	"os"
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

type State struct {
	nil       *Obj // nil: ()
	readStack *Obj // defered obj to emit from read

	// Atoms
	internedAtoms *Obj // interned atoms list
	atomTrue      *Obj // atom: t
	atomQuote     *Obj // atom: quote
	atomPush      *Obj // atom: push
	atomPop       *Obj // atom: pop

	// stack/env
	stack *Obj // top-of-stack (implemented with pairs)
	env   *Obj // top-level / initial environment

	input string // input data string used by read()
	pos   uint64 // input data position used by read()
}

var state State

func nilNew() *Obj {
	return &Obj{Tag: TagNil, Nil: &Nil{}}
}

func atomNew(str string) *Obj {
	return &Obj{Tag: TagAtom, Atom: ptr(Atom(str))}
}

func numberNew(n int64) *Obj {
	return &Obj{Tag: TagNumber, Number: ptr(Number(n))}
}

func pairNew(car, cdr *Obj) *Obj {
	return &Obj{Tag: TagPair, Pair: &Pair{car: car, cdr: cdr}}
}

func closureNew(body, env *Obj) *Obj {
	return &Obj{Tag: TagClosure, Closure: &Closure{body: body, env: env}}
}

func primitiveNew(f func(env **Obj)) *Obj {
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

func intern(atomBuf string) *Obj {
	for list := state.internedAtoms; list != state.nil; list = list.cdr {
		assertTag(list, TagPair, fmt.Sprintf("state.interned_atoms must be Pairs got %v", list))

		elem := list.car
		assertTag(elem, TagAtom, fmt.Sprintf("state.interned_atoms.car must be an Atom got %v", elem))
		if len(atomBuf) == len(*elem.Atom) && atomBuf == string(*elem.Atom) {
			return elem
		}
	}

	// not found, create a new one and push the front of the list
	atom := atomNew(atomBuf)
	state.internedAtoms = pairNew(atom, state.internedAtoms)

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

func objEqual(a *Obj, b *Obj) bool {
	return *a == *b || (is(a, TagNumber) && is(b, TagNumber) && *a.Number == *b.Number)
}

func objInt64(a *Obj) int64 {
	if is(a, TagNumber) {
		return int64(*a.Number)
	}

	return 0
}

func peek() uint8 {
	if state.pos == uint64(len(state.input)) {
		return 0
	}

	return state.input[state.pos]
}

func advance() {
	assert(peek() != 0, "cannot advance further")
	state.pos++
}

func isWhite(c uint8) bool { return c == ' ' || c == '\t' || c == '\n' }

func isDirective(c uint8) bool { return c == '\'' || c == '^' || c == '$' }

func isPunctuation(c uint8) bool {
	return c == 0 || isWhite(c) || isDirective(c) || c == '(' || c == ')' || c == ';'
}

func skipWhiteAndComments() {
	c := peek()
	if c == 0 {
		return
	}

	// skip whitespace
	if isWhite(c) {
		advance()
		skipWhiteAndComments()
		return
	}

	// skip comment
	if c == ';' {
		advance()
		for {
			c = peek()
			if c == 0 {
				return
			}
			advance()
			if c == '\n' {
				break
			}
		}

		skipWhiteAndComments()
		return
	}
}

func readList() *Obj {
	if state.readStack == nil {
		skipWhiteAndComments()
		c := peek()
		if c == ')' {
			advance()
			return state.nil
		}
	}

	first := read()
	second := readList()
	return pairNew(first, second)
}

func parseInt64(str string) (int64, bool) {
	i, err := strconv.ParseInt(str, 10, 64)
	return i, err == nil
}

func readScalar() *Obj {
	// otherwise, assume atom or number and read it
	start := state.pos
	for !isPunctuation(peek()) {
		advance()
	}

	str := state.input[start:state.pos]
	// is it a number?
	if n, ok := parseInt64(str); ok {
		return numberNew(n)
	}

	// atom
	return intern(str)
}

func read() *Obj {
	read_stack := state.readStack
	if read_stack != nil {
		state.readStack = cdr(read_stack)
		return car(read_stack)
	}

	skipWhiteAndComments()

	c := peek()
	switch c {
	case 0:
		fail("End of input: could not read()")

	// A quote?
	case '\'':
		advance()
		return state.atomQuote

	// A push?
	case '^':
		advance()
		var s *Obj
		s = pairNew(state.atomPush, s)
		s = pairNew(readScalar(), s)
		s = pairNew(state.atomQuote, s)
		state.readStack = s

		return read()

	// A pop?
	case '$':
		advance()
		var s *Obj
		s = pairNew(state.atomPop, s)
		s = pairNew(readScalar(), s)
		s = pairNew(state.atomQuote, s)
		state.readStack = s

		return read()

	// Read a list?
	case '(':
		advance()
		return readList()

	}

	return readScalar()
}

func printListTail(obj *Obj) {
	if obj == state.nil {
		fmt.Print(")")
		return
	}

	if is(obj, TagPair) {
		fmt.Print(" ")
		printRecurse(obj.car)
		printListTail(obj.cdr)
	} else {
		fmt.Print(" . ")
		printRecurse(obj)
		fmt.Print(")")
	}
}

func printRecurse(obj *Obj) {
	if obj == state.nil {
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
		printRecurse(obj.car)
		printListTail(obj.cdr)

	case TagClosure:
		fmt.Print("CLOSURE<")
		printRecurse(obj.body)
		fmt.Printf(", %p>", obj.env)

	case TagPrimitive:
		fmt.Printf("PRIM<%p>", obj.Primitive)
	}
}

func print(obj *Obj) {
	printRecurse(obj)
	fmt.Println()
}

func envFind(env *Obj, key *Obj) *Obj {
	if !is(key, TagAtom) {
		failf("Expected 'key' to be an Atom in env_find() got %v", key)
	}

	for v := env; v != state.nil; v = cdr(v) {
		kv := car(v)
		if key == car(kv) || *key == *car(kv) {
			return cdr(kv)
		}
	}

	failf("Failed to find key='%s' in environment", *key.Atom)
	return nil
}

func envDefine(env *Obj, key *Obj, val *Obj) *Obj {
	return pairNew(pairNew(key, val), env)
}

func envDefinePrim(env *Obj, name string, fn func(env **Obj)) *Obj {
	return envDefine(env, intern(name), primitiveNew(fn))
}

func push(obj *Obj) {
	state.stack = pairNew(obj, state.stack)
}

func tryPop() (*Obj, bool) {
	if state.stack == state.nil {
		return nil, false
	}

	o := car(state.stack)
	state.stack = cdr(state.stack)
	return o, true
}

func pop() *Obj {
	if ret, ok := tryPop(); ok {
		return ret
	}

	fail("Value Stack Underflow")
	return nil
}

func compute(comp *Obj, env *Obj) {
	for comp != state.nil {
		cmd := car(comp)
		comp = cdr(comp)

		if cmd == state.atomQuote {
			if comp == state.nil {
				fail("Expected data following a quote form")
			}
			push(car(comp))
			comp = cdr(comp)

			continue
		}

		eval(cmd, &env)
	}
}

func eval(expr *Obj, env **Obj) {
	if is(expr, TagAtom) {
		val := envFind(*env, expr)
		if is(val, TagClosure) {
			compute(val.body, val.env)
		} else if is(val, TagPrimitive) {
			(*val.Primitive)(env)
		} else {
			push(val)
		}
	} else if is(expr, TagNil) || is(expr, TagPair) {
		push(closureNew(expr, *env))
	} else {
		push(expr)
	}
}

// Core primitives
func primPush(env **Obj) { push(envFind(*env, pop())) }

func primPop(env **Obj) {
	k, v := pop(), pop()
	*env = envDefine(*env, k, v)
}

func primEq(_ **Obj) {
	if objEqual(pop(), pop()) {
		push(state.atomTrue)
	} else {
		push(state.nil)
	}
}

func primCons(_ **Obj) {
	a, b := pop(), pop()
	push(pairNew(a, b))
}

func primCar(_ **Obj) { push(car(pop())) }
func primCdr(_ **Obj) { push(cdr(pop())) }

func primCswap(_ **Obj) {
	if pop() == state.atomTrue {
		a, b := pop(), pop()
		push(a)
		push(b)
	}
}

func primTag(_ **Obj)   { push(numberNew(int64(pop().Tag))) }
func primRead(_ **Obj)  { push(read()) }
func primPrint(_ **Obj) { print(pop()) }

// Extra primitives
func primStack(_ **Obj) { push(state.stack) }
func primEnv(env **Obj) { push(*env) }

func primSub(_ **Obj) {
	b, a := pop(), pop()
	push(numberNew(objInt64(a) - objInt64(b)))
}

func primMul(_ **Obj) {
	b, a := pop(), pop()
	push(numberNew(objInt64(a) * objInt64(b)))
}

func primNand(_ **Obj) {
	b, a := pop(), pop()
	push(numberNew(^(objInt64(a) & objInt64(b))))
}

func primLsh(_ **Obj) {
	b, a := pop(), pop()
	push(numberNew(objInt64(a) << uint(objInt64(b))))
}

func primRsh(_ **Obj) {
	b, a := pop(), pop()
	push(numberNew(objInt64(a) >> uint(objInt64(b))))
}

func loadFile(filename string) (string, bool) {
	file, err := os.Open(filename)
	if err != nil {
		return "", false
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return "", false
	}

	return string(b), true
}

func setup(filename string) {
	if input, ok := loadFile(filename); ok {
		state.input = input
	} else {
		panic("failed to load input file")
	}

	state.pos = 0

	state.readStack = nil
	state.nil = nilNew()

	state.internedAtoms = state.nil
	state.atomTrue = intern("t")
	state.atomQuote = intern("quote")
	state.atomPush = intern("push")
	state.atomPop = intern("pop")

	state.stack = state.nil

	env := state.nil

	// core primitives
	env = envDefinePrim(env, "push", primPush)
	env = envDefinePrim(env, "pop", primPop)
	env = envDefinePrim(env, "cons", primCons)
	env = envDefinePrim(env, "car", primCar)
	env = envDefinePrim(env, "cdr", primCdr)
	env = envDefinePrim(env, "eq", primEq)
	env = envDefinePrim(env, "cswap", primCswap)
	env = envDefinePrim(env, "tag", primTag)
	env = envDefinePrim(env, "read", primRead)
	env = envDefinePrim(env, "print", primPrint)

	// extra primitives
	env = envDefinePrim(env, "stack", primStack)
	env = envDefinePrim(env, "env", primEnv)
	env = envDefinePrim(env, "-", primSub)
	env = envDefinePrim(env, "*", primMul)
	env = envDefinePrim(env, "nand", primNand)
	env = envDefinePrim(env, "<<", primLsh)
	env = envDefinePrim(env, ">>", primRsh)

	state.env = env
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %s path\n", os.Args[0])
		os.Exit(1)
	}

	setup(os.Args[1])

	obj := read()
	compute(obj, state.env)
}
