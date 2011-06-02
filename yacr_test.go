package yacr

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func makeReader(s string) *Reader {
	return NewReader(strings.NewReader(s), ',', false)
}

func checkValueCount(t *testing.T, expected int, values [][]byte) {
	if len(values) != expected {
		t.Errorf("Expected %d value(s), but got %d (%#v)", expected, len(values), values)
	}
}

func checkNoError(t *testing.T, e os.Error) {
	if e != nil {
		t.Error(e)
	}
}

func TestSingleValue(t *testing.T) {
	r := makeReader("Foo")
	values, e := r.ReadRow()
	checkNoError(t, e)
	checkValueCount(t, 1, values)
	values, e = r.ReadRow()
	if values != nil {
		t.Errorf("No value expected, but got %#v", values)
	}
	if e == nil {
		t.Error("EOF expected")
	}
	if e != os.EOF {
		t.Error(e)
	}
}

func TestTwoValues(t *testing.T) {
	r := makeReader("Foo,Bar")
	values, e := r.ReadRow()
	checkNoError(t, e)
	checkValueCount(t, 2, values)
	expected := [][]byte{[]byte("Foo"), []byte("Bar")}
	if !reflect.DeepEqual(expected, values) {
		t.Errorf("Expected %#v, got %#v", expected, values)
	}
}

func TestTwoLines(t *testing.T) {
	row1 := strings.Repeat("1,2,3,4,5,6,7,8,9,10,", 5)
	row2 := strings.Repeat("a,b,c,d,e,f,g,h,i,j,", 3)
	content := strings.Join([]string{row1, row2}, "\n")
	r := makeReader(content)
	values, e := r.ReadRow()
	checkNoError(t, e)
	checkValueCount(t, 51, values)
	values, e = r.ReadRow()
	checkNoError(t, e)
	checkValueCount(t, 31, values)
}

func TestLongLine(t *testing.T) {
	content := strings.Repeat("1,2,3,4,5,6,7,8,9,10,", 200)
	r := makeReader(content)
	values, e := r.ReadRow()
	checkNoError(t, e)
	checkValueCount(t, 2001, values)
}

func BenchmarkParsing(b *testing.B) {
	b.StopTimer()
	str := strings.Repeat("aaaaaaaa,b b b b b b b,cc cc cc cc cc, ddddd ddd\n", 2000)
	b.SetBytes(int64(len(str)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r := makeReader(str)
		nb := 0
		for {
			_, e := r.ReadRow()
			if e == os.EOF {
				break
			}
			if e != nil {
				panic(e.String())
			}
			nb++
		}
		if nb != 2000 {
			panic("wrong # rows")
		}
	}
}