package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/losinggeneration/forsp-go/forsp"
)

type forspError struct{ error }

func loadFile(filename string) (io.Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b), nil
}

type Forsp struct {
	*forsp.Forsp

	done bool
}

func (f *Forsp) primBye(_ **forsp.Obj) { f.done = true }

func New() *Forsp {
	f := Forsp{
		Forsp: forsp.New(),
	}

	f.Env = f.EnvDefinePrim(f.Env, "bye", f.primBye)

	return &f
}

func run(f *Forsp, i int) error {
	r, err := loadFile(os.Args[i])
	if err != nil {
		return err
	}

	if err := f.SetReader(r); err != nil {
		return err
	}

	obj := f.Read()
	f.Compute(obj)

	return nil
}

func repl(f *Forsp, reader *bufio.Reader) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = forspError{fmt.Errorf("%v", r)}
		}
	}()

	line, _, err := reader.ReadLine()
	if err != nil {
		return err
	}

	if err := f.SetReader(strings.NewReader(string(line))); err != nil {
		return err
	}

	obj := f.Read()
	f.Compute(obj)

	return nil
}

func main() {
	f := New()

	if len(os.Args) >= 2 {
		for i := 1; i < len(os.Args); i++ {
			err := run(f, i)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		return
	}

	reader := bufio.NewReader(os.Stdin)
	for !f.done {
		if err := repl(f, reader); !errors.Is(err, forspError{}) {
			fmt.Fprintln(os.Stderr, err)
		} else if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
