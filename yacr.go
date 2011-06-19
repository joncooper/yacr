// The author disclaims copyright to this source code.
package yacr

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type Reader struct {
	sep    byte
	quoted bool
	//trim	bool
	b      *bufio.Reader
	rd     io.Reader
	buf    []byte
	values [][]byte
}

func DefaultReader(rd io.Reader) *Reader {
	return NewReader(rd, ',', true)
}
func DefaultFileReader(filepath string) (*Reader, os.Error) {
	return NewFileReader(filepath, ',', true)
}
func NewReaderBytes(b []byte, sep byte, quoted bool) *Reader {
	return NewReader(bytes.NewBuffer(b), sep, quoted)
}
func NewReaderString(s string, sep byte, quoted bool) *Reader {
	return NewReader(strings.NewReader(s), sep, quoted)
}
func NewReader(rd io.Reader, sep byte, quoted bool) *Reader {
	return &Reader{sep: sep, quoted: quoted, b: bufio.NewReader(rd), rd: rd, values: make([][]byte, 20)}
}
func NewFileReader(filepath string, sep byte, quoted bool) (*Reader, os.Error) {
	rd, err := zopen(filepath)
	if err != nil {
		return nil, err
	}
	return NewReader(rd, sep, quoted), nil
}

func (r *Reader) Close() os.Error {
	c, ok := r.rd.(io.Closer)
	if ok {
		return c.Close()
	}
	return nil
}

func (r *Reader) ReadRow() ([][]byte, os.Error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	if r.quoted {
		start := 0
		values, isPrefix := r.scanLine(line, false)
		for isPrefix {
			start = deepCopy(values, start)
			line, err := r.readLine()
			if err != nil {
				return nil, err
			}
			values, isPrefix = r.scanLine(line, true)
		}
		return values, nil
	}
	return r.split(line), nil
}

func (r *Reader) scanLine(line []byte, continuation bool) ([][]byte, bool) {
	start := 0
	var a [][]byte
	if continuation {
		a = r.values
	} else {
		a = r.values[0:0]
	}
	quotedChunk := continuation
	endQuotedChunk := -1
	escapedQuotes := 0
	var chunk []byte
	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			if quotedChunk {
				if i < (len(line)-1) && line[i+1] == '"' {
					escapedQuotes += 1
					i++
				} else {
					quotedChunk = false
					endQuotedChunk = i
				}
			} else if i == 0 || line[i-1] == r.sep {
				quotedChunk = true
				start = i + 1
			}
		} else if line[i] == r.sep && !quotedChunk {
			if endQuotedChunk >= 0 {
				chunk = unescapeQuotes(line[start:endQuotedChunk], escapedQuotes)
				escapedQuotes = 0
				endQuotedChunk = -1
			} else {
				chunk = line[start:i]
			}
			if continuation {
				fixLastChunk(a, chunk)
				continuation = false
			} else {
				a = append(a, chunk)
			}
			start = i + 1
		}
	}
	if endQuotedChunk >= 0 {
		chunk = unescapeQuotes(line[start:endQuotedChunk], escapedQuotes)
	} else {
		chunk = unescapeQuotes(line[start:], escapedQuotes)
	}
	if continuation {
		fixLastChunk(a, chunk)
	} else {
		a = append(a, chunk)
	}
	r.values = a // if cap(a) != cap(r.values)
	return a, quotedChunk
}

func unescapeQuotes(b []byte, count int) []byte {
	if count == 0 {
		return b
	}
	c := make([]byte, len(b)-count)
	for i, j := 0, 0; i < len(b); i, j = i+1, j+1 {
		c[j] = b[i]
		if b[i] == '"' {
			i++
		}
	}
	return c
}

func fixLastChunk(values [][]byte, continuation []byte) {
	prefix := values[len(values)-1]
	prefix = append(prefix, '\n') // TODO \r\n ?
	prefix = append(prefix, continuation...)
	values[len(values)-1] = prefix
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
		if buf == nil {
			if !isPrefix {
				return line, nil
			}
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

type Writer struct {
	sep    byte
	quoted bool
	//trim	bool
	b *bufio.Writer
}

func DefaultWriter(wr io.Writer) *Writer {
	return NewWriter(wr, ',', true)
}
func NewWriter(wr io.Writer, sep byte, quoted bool) *Writer {
	return &Writer{sep: sep, quoted: quoted, b: bufio.NewWriter(wr)}
}

func (w *Writer) WriteRow(row [][]byte) (err os.Error) {
	for i, v := range row {
		if i > 0 {
			err = w.b.WriteByte(w.sep)
			if err != nil {
				return
			}
		}
		err = w.write(v)
		if err != nil {
			return
		}
	}
	err = w.b.WriteByte('\n') // TODO \r\n ?
	if err != nil {
		return
	}
	return
}

func (w *Writer) Flush() os.Error {
	return w.b.Flush()
}

func (w *Writer) write(value []byte) (err os.Error) {
	// In quoted mode, value is enclosed between quotes if it contains sep, quote or \n.
	if w.quoted {
		last := 0
		for i, c := range value {
			switch c {
			case '"', '\n', w.sep:
			default:
				continue
			}
			if last == 0 {
				err = w.b.WriteByte('"')
				if err != nil {
					return
				}
			}
			_, err = w.b.Write(value[last:i])
			if err != nil {
				return
			}
			err = w.b.WriteByte(c)
			if err != nil {
				return
			}
			if c == '"' {
				err = w.b.WriteByte(c)
				if err != nil {
					return
				}
			}
			last = i + 1
		}
		_, err = w.b.Write(value[last:])
		if err != nil {
			return
		}
		if last != 0 {
			err = w.b.WriteByte('"')
		}
	} else {
		_, err = w.b.Write(value)
	}
	return
}

func deepCopy(row [][]byte, start int) int {
	var dup []byte
	for i := start; i < len(row); i++ {
		dup = make([]byte, len(row[i]))
		copy(dup, row[i])
		row[i] = dup
	}
	return len(row)
}

func DeepCopy(row [][]byte) [][]byte {
	dup := make([][]byte, len(row))
	for i := 0; i < len(row); i++ {
		dup[i] = make([]byte, len(row[i]))
		copy(dup[i], row[i])
	}
	return dup
}

type zReadCloser struct {
	f  *os.File
	rd io.ReadCloser
}

// TODO Create golang bindings for zlib (gzopen) or libarchive?
func zopen(filepath string) (io.ReadCloser, os.Error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	var rd io.ReadCloser
	// TODO zip, bz2
	ext := path.Ext(f.Name())
	if ext == ".gz" {
		rd, err = gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
	} else if ext == ".bz2" {
		rd = ioutil.NopCloser(bzip2.NewReader(f))
	} else {
		rd = f
	}
	return &zReadCloser{f, rd}, nil
}
func (z *zReadCloser) Read(b []byte) (n int, err os.Error) {
	return z.rd.Read(b)
}
func (z *zReadCloser) Close() (err os.Error) {
	err = z.rd.Close()
	if err != nil {
		return
	}
	return z.f.Close()
}
