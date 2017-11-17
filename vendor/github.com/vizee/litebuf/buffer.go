package litebuf

import (
	"strconv"
	"unicode/utf8"
)

const (
	hexdigits = "0123456789abcdef"

	noescchr = '0'
	esctable = `00000000btn0fr00000000000000000000"000000000000/00000000000000000000000000000000000000000000\00000000000000000000000000000000000`

	cacheAlign   = 64 // cache line size
	preallocSize = 32 // preallocSize >= (cacheAlign / 2)

	pageSize = 4096 // page size
	pageMask = pageSize - 1
)

func pageRound(n int) int {
	return (n + pageSize - 1) &^ pageMask
}

type Buffer struct {
	buf []byte
	p   int
	pre [preallocSize]byte
}

func (b *Buffer) Len() int {
	return b.p
}

func (b *Buffer) Cap() int {
	return cap(b.buf)
}

func (b *Buffer) Bytes() []byte {
	return b.buf[:b.p]
}

func (b *Buffer) String() string {
	return string(b.Bytes())
}

func (b *Buffer) Reset() {
	b.p = 0
	b.buf = b.buf[:cap(b.buf)]
}

func (b *Buffer) Trim(n int) {
	if b.p > n {
		b.p -= n
	} else {
		b.p = 0
	}
}

func (b *Buffer) Resize(n int) {
	if cap(b.buf) >= n {
		b.buf = b.buf[:n]
		if b.p > n {
			b.p = n
		}
	} else if n <= preallocSize {
		copy(b.pre[:], b.buf[:b.p])
		b.buf = b.pre[:]
	} else {
		if n <= pageSize/2 {
			n = (n + n) &^ (cacheAlign - 1)
		} else {
			n = pageRound(n + cap(b.buf)/2)
		}
		buf := make([]byte, n)
		copy(buf, b.buf[:b.p])
		b.buf = buf
	}
}

func (b *Buffer) Reserve(n int) []byte {
	if b.p+n > len(b.buf) {
		b.Resize(b.p + n)
	}
	p := b.p
	b.p += n
	return b.buf[p:b.p]
}

func (b *Buffer) AppendInt(i int64, base int) {
	b.buf = strconv.AppendInt(b.buf[:b.p], i, base)
	b.p = len(b.buf)
}

func (b *Buffer) AppendUint(u uint64, base int) {
	b.buf = strconv.AppendUint(b.buf[:b.p], u, base)
	b.p = len(b.buf)
}

func (b *Buffer) AppendFloat(f float64, fmt byte, p int, bits int) {
	b.buf = strconv.AppendFloat(b.buf[:b.p], f, fmt, p, bits)
	b.p = len(b.buf)
}

func (b *Buffer) WriteByte(c byte) {
	if b.p >= len(b.buf) {
		b.Resize(b.p + 8)
	}
	b.buf[b.p] = c
	b.p++
}

func (b *Buffer) WriteQuote(s string, unicode bool) {
	if b.p+len(s)+2 > len(b.buf) {
		b.Resize(b.p + len(s) + 2)
	}
	b.buf[b.p] = '"'
	b.p++
	p := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < utf8.RuneSelf {
			if esctable[c] != noescchr {
				if n := i - p + 2; b.p+n > len(b.buf) {
					b.Resize(b.p + n)
				}
				b.p += copy(b.buf[b.p:], s[p:i])
				b.buf[b.p] = '\\'
				b.buf[b.p+1] = esctable[c]
				b.p += 2
				p = i + 1
			}
		} else if unicode {
			if n := i - p + 6; b.p+n > len(b.buf) {
				b.Resize(b.p + n)
			}
			b.p += copy(b.buf[b.p:], s[p:i])
			r, n := utf8.DecodeRuneInString(s[i:])
			buf := b.buf[b.p:]
			b.p += 6
			_ = buf[5]
			buf[0] = '\\'
			buf[1] = 'u'
			buf[2] = hexdigits[(r>>12)&0xf]
			buf[3] = hexdigits[(r>>8)&0xf]
			buf[4] = hexdigits[(r>>4)&0xf]
			buf[5] = hexdigits[r&0xf]
			i += n - 1
			p = i + 1
		}
	}
	if n := len(s) - p + 1; b.p+n > len(b.buf) {
		b.Resize(b.p + n)
	}
	b.p += copy(b.buf[b.p:], s[p:])
	b.buf[b.p] = '"'
	b.p++
}

func (b *Buffer) WriteString(s string) (int, error) {
	if b.p+len(s) > len(b.buf) {
		b.Resize(b.p + len(s))
	}
	n := copy(b.buf[b.p:b.p+len(s)], s)
	b.p += n
	return n, nil
}

func (b *Buffer) Write(p []byte) (int, error) {
	if b.p+len(p) > len(b.buf) {
		b.Resize(b.p + len(p))
	}
	n := copy(b.buf[b.p:b.p+len(p)], p)
	b.p += n
	return n, nil
}
