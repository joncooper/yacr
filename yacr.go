package yacr

import (
	"bufio"
	"io"
	"os"
)

type Reader struct {
	sep    byte
	quotes bool
	//trim	bool
	b      *bufio.Reader
	buf    []byte
	values [][]byte
}

func DefaultReader(rd io.Reader) *Reader {
	return NewReader(rd, ',', true)
}
func NewReader(rd io.Reader, sep byte, quotes bool) *Reader {
	return &Reader{sep: sep, quotes: quotes, b: bufio.NewReader(rd), values: make([][]byte, 20)}
}

func (r *Reader) ReadRow() ([][]byte, os.Error) {
	// TODO
	if r.quotes {
		panic("Quoted mode not supported yet")
	}
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return r.split(line), nil
}

func (r *Reader) readLine() ([]byte, os.Error) {
	var buf, line []byte
	var err os.Error
	isPrefix := true
	for isPrefix {
		line, isPrefix, err = r.b.ReadLine()
		if err != nil {
			return nil, err
		}
		if !isPrefix {
			if buf == nil {
				return line, nil
			}
		}
		if buf == nil {
			buf = r.buf[0:0]
		}
		buf = append(buf, line...)
	}
	r.buf = buf // if cap(buf) != cap(r.buf)
	return buf, nil
}

func (r *Reader) split(line []byte) [][]byte {
	start := 0
	a := r.values[0:0]
	for i := 0; i < len(line); i++ {
		if line[i] == r.sep {
			a = append(a, line[start:i])
			start = i + 1
		}
	}
	a = append(a, line[start:])
	r.values = a // if cap(a) != cap(r.values)
	return a
}