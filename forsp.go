package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/losinggeneration/forsp-go/forsp"
)

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

func main() {
	f := New()

	if len(os.Args) >= 2 {
		r, err := loadFile(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if err := f.SetReader(r); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		obj := f.Read()
		f.Compute(obj)

		return
	}

	reader := bufio.NewReader(os.Stdin)
	for !f.done {
		line, _, err := reader.ReadLine()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if err := f.SetReader(strings.NewReader(string(line))); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		obj := f.Read()
		f.Compute(obj)
	}
}
