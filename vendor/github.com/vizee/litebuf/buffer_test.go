package litebuf

import (
	"bytes"
	"reflect"
	"testing"
	"unicode/utf8"
)

func TestGenerateTable(t *testing.T) {
	table := make([]byte, 128)
	for i := range table {
		table[i] = noescchr
	}
	escapechars := "\"\\/\b\f\n\r\t"
	escapeto := `"\/bfnrt`
	for i := range escapechars {
		table[escapechars[i]] = escapeto[i]
	}
	t.Log(string(table))
}

func TestBufferStruct(t *testing.T) {
	rt := reflect.TypeOf(Buffer{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		t.Logf("%-8s %4d %4d", f.Name, f.Offset, f.Type.Size())
	}
}

func TestWrite(t *testing.T) {
	buf := new(Buffer)
	t.Log(buf.Len(), buf.Cap())

	// WriteString
	buf.WriteString("ab")
	t.Log(buf.Len(), buf.Cap())

	// Write
	buf.Write([]byte{'c', 'd', 'e'})
	t.Log(buf.Len(), buf.Cap())

	buf.WriteString("fghij")
	t.Log(buf.Len(), buf.Cap())

	// WriteByte
	buf.WriteByte('k')
	t.Log(buf.Len(), buf.Cap())

	// Reserve
	resv := buf.Reserve(buf.Cap() - buf.Len() + 1)
	t.Log("reserve", len(resv))
	for i := 0; i < len(resv); i++ {
		resv[i] = 'A' + byte(i)
	}
	t.Log(buf.Len(), buf.Cap())

	// Stringer
	t.Log(buf)

	// valid
	should := "abcdefghijkABCDEFGHIJKLMNOPQRSTUV"
	if !bytes.Equal(buf.Bytes(), []byte(should)) {
		t.Fatal("valid failed")
	}

	buf.Resize(0)
	t.Log(buf.Len(), buf.Cap())

	buf.Reset()
	t.Log(buf.Len(), buf.Cap())
}

func quoteString(buf *Buffer, s string, unicode bool) {
	buf.WriteByte('"')
	p := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < utf8.RuneSelf {
			if esctable[c] != noescchr {
				if p < i {
					buf.WriteString(s[p:i])
				}
				buf.WriteByte('\\')
				buf.WriteByte(esctable[c])
				p = i + 1
			}
		} else if unicode {
			if p < i {
				buf.WriteString(s[p:i])
			}
			r, n := utf8.DecodeRuneInString(s[i:])
			h := [6]byte{
				'\\',
				'u',
				hexdigits[(r>>12)&0xf],
				hexdigits[(r>>8)&0xf],
				hexdigits[(r>>4)&0xf],
				hexdigits[(r)&0xf],
			}
			buf.Write(h[:])
			i += n - 1
			p = i + 1
		}
	}
	if p < len(s) {
		buf.WriteString(s[p:])
	}
	buf.WriteByte('"')
}

func TestWriteQuote(t *testing.T) {
	quotetest := func(s string, unicode bool) {
		buf := Buffer{}
		buf.WriteQuote(s, unicode)
		s1 := buf.String()
		buf.Reset()
		quoteString(&buf, s, unicode)
		s2 := buf.String()
		if s1 != s2 {
			t.Error("error case", s, s1, s2)
		}
		t.Log(s1)
	}
	quotetest("abc", false)
	quotetest("abc\ncde", false)
	quotetest("你好世界", false)
	quotetest("你好\n世界", true)
}

func BenchmarkGoBuffer(b *testing.B) {
	b.ReportAllocs()
	buf := bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Write([]byte{'a', 'b', 'c', 'd', 'e'})
	}
	b.Log(buf.Len(), buf.Cap())
}

func BenchmarkLiteBuffer(b *testing.B) {
	b.ReportAllocs()
	buf := Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Write([]byte{'a', 'b', 'c', 'd', 'e'})
	}
	b.Log(buf.Len(), buf.Cap())
}

func BenchmarkWriteQuote(b *testing.B) {
	buf := Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.WriteQuote("你好世界", true)
	}
}

func BenchmarkQuoteString(b *testing.B) {
	buf := Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		quoteString(&buf, "你好世界", true)
	}
}
