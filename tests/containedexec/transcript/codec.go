// Exact conformance DEFLATE transcript, per RFC 1951.
// Records original Huffman headers and every symbol/extra bit; no match search.
package main

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"math/rand"
)

func ck(ok bool) {
	if !ok {
		panic("invalid or oversized transcript")
	}
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}

type br struct {
	b []byte
	p int
}

func (r *br) peek(n int) uint32 {
	ck(n >= 0 && n <= 16)
	i := r.p / 8
	var v uint32
	for j := 0; j < 3 && i+j < len(r.b); j++ {
		v |= uint32(r.b[i+j]) << uint(8*j)
	}
	return (v >> uint(r.p%8)) & ((1 << uint(n)) - 1)
}
func (r *br) get(n int) uint32 { ck(r.p+n <= len(r.b)*8); v := r.peek(n); r.p += n; return v }

type bw struct {
	b   []byte
	p   int
	acc uint64
	n   uint
	off int
}

func (w *bw) put(v uint32, n int) {
	ck(n >= 0 && n <= 16 && v < 1<<uint(n) && w.p+n <= len(w.b)*8)
	w.acc |= uint64(v) << w.n
	w.n += uint(n)
	w.p += n
	if w.n >= 32 {
		binary.LittleEndian.PutUint32(w.b[w.off:], uint32(w.acc))
		w.off += 4
		w.acc >>= 32
		w.n -= 32
	}
}
func (w *bw) finish() {
	for w.n > 0 {
		w.b[w.off] = byte(w.acc)
		w.off++
		w.acc >>= 8
		if w.n <= 8 {
			w.n = 0
		} else {
			w.n -= 8
		}
	}
}
func (w *bw) copyBits(r *br, n int) {
	for n > 0 {
		k := min(n, 16)
		w.put(r.get(k), k)
		n -= k
	}
}
func bitSlice(b []byte, p, n int) []byte {
	w := bw{b: make([]byte, (n+7)/8)}
	w.copyBits(&br{b, p}, n)
	w.finish()
	return w.b
}

type tree struct {
	lens  []int
	codes []uint32
	table []uint32
	width int
}

func makeTree(l []int, decode bool) tree {
	t := tree{lens: l, codes: make([]uint32, len(l))}
	var counts, next [16]int
	for _, n := range l {
		ck(n >= 0 && n <= 15)
		if n > 0 {
			counts[n]++
			t.width = max(t.width, n)
		}
	}
	code := 0
	for n := 1; n <= 15; n++ {
		code = (code + counts[n-1]) * 2
		next[n] = code
		ck(code+counts[n] <= 1<<uint(n))
	}
	if decode {
		t.table = make([]uint32, 1<<uint(t.width))
	}
	for s, n := range l {
		if n == 0 {
			continue
		}
		c := bits.Reverse32(uint32(next[n])) >> uint(32-n)
		next[n]++
		t.codes[s] = c
		if decode {
			for i := int(c); i < len(t.table); i += 1 << uint(n) {
				t.table[i] = uint32(n<<16 | s)
			}
		}
	}
	return t
}
func (t tree) read(r *br) int {
	ck(t.width > 0)
	v := t.table[r.peek(t.width)]
	n := int(v >> 16)
	ck(n > 0)
	r.get(n)
	return int(v & 65535)
}
func (t *tree) write(w *bw, s int) {
	ck(s >= 0 && s < len(t.lens) && t.lens[s] > 0)
	w.put(t.codes[s], t.lens[s])
}

var le = []int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}
var de = []int{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13}

// The caller has already consumed the BFINAL/BTYPE bits.
func trees(r *br, typ uint32, decode bool) (tree, tree) {
	ll := make([]int, 288)
	dd := make([]int, 32)
	if typ == 1 {
		for i := range ll {
			switch {
			case i < 144:
				ll[i] = 8
			case i < 256:
				ll[i] = 9
			case i < 280:
				ll[i] = 7
			default:
				ll[i] = 8
			}
		}
		for i := range dd {
			dd[i] = 5
		}
	} else {
		ck(typ == 2)
		nl, nd, nc := int(r.get(5))+257, int(r.get(5))+1, int(r.get(4))+4
		ck(nl <= 286)
		order := []int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}
		cl := make([]int, 19)
		for i := 0; i < nc; i++ {
			cl[order[i]] = int(r.get(3))
		}
		ct := makeTree(cl, true)
		all := make([]int, 0, nl+nd)
		for len(all) < nl+nd {
			s := ct.read(r)
			if s < 16 {
				all = append(all, s)
				continue
			}
			n, v := 0, 0
			switch s {
			case 16:
				ck(len(all) > 0)
				v = all[len(all)-1]
				n = int(r.get(2)) + 3
			case 17:
				n = int(r.get(3)) + 3
			case 18:
				n = int(r.get(7)) + 11
			default:
				panic("code length")
			}
			ck(len(all)+n <= nl+nd)
			for i := 0; i < n; i++ {
				all = append(all, v)
			}
		}
		ll = all[:nl]
		dd = all[nl:]
	}
	ck(ll[256] > 0)
	return makeTree(ll, decode), makeTree(dd, decode)
}
func u32(w *bytes.Buffer, n int) {
	ck(n >= 0 && n <= 64<<20)
	must(binary.Write(w, binary.LittleEndian, uint32(n)))
}
func blob(w *bytes.Buffer, b []byte) { u32(w, len(b)); w.Write(b) }

type input struct {
	b []byte
	p int
}

func (r *input) take(n int) []byte {
	ck(n >= 0 && n <= len(r.b)-r.p)
	b := r.b[r.p : r.p+n]
	r.p += n
	return b
}
func (r *input) num() int     { n := int(binary.LittleEndian.Uint32(r.take(4))); ck(n <= 64<<20); return n }
func (r *input) blob() []byte { return r.take(r.num()) }
func words(a []uint16) []byte {
	b := make([]byte, len(a)*2)
	for i, v := range a {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return b
}
func packZ(b []byte) []byte {
	ck(len(b) >= 6 && len(b) <= 16<<20 && b[0]&15 == 8 && b[1]&32 == 0 && (int(b[0])*256+int(b[1]))%31 == 0)
	var out bytes.Buffer
	out.WriteString("ZT01")
	u32(&out, len(b))
	sum := sha256.Sum256(b)
	out.Write(sum[:])
	out.Write(b[:2])
	r := br{b: b, p: 16}
	for blocks := 0; ; blocks++ {
		ck(blocks < 65536)
		start := r.p
		final, typ := r.get(1), r.get(2)
		if typ == 0 {
			r.get((8 - r.p%8) % 8)
			n := int(r.get(16))
			nn := r.get(16)
			ck(uint32(n)^nn == 65535)
			r.p += n * 8
			ck(r.p <= len(b)*8)
			out.WriteByte(0)
			u32(&out, r.p-start)
			blob(&out, bitSlice(b, start, r.p-start))
		} else {
			ll, dd := trees(&r, typ, true)
			hn := r.p - start
			header := bitSlice(b, start, hn)
			syms := make([]uint16, 0, 32768)
			extra := make([]uint16, 0, 32768)
			for {
				s := ll.read(&r)
				syms = append(syms, uint16(s))
				ck(len(syms) <= 16<<20)
				if s == 256 {
					break
				}
				if s > 256 {
					ck(s <= 285)
					x := r.get(le[s-257])
					d := dd.read(&r)
					ck(d < 30)
					y := r.get(de[d])
					extra = append(extra, uint16(x), uint16(d), uint16(y))
				}
			}
			out.WriteByte(1)
			u32(&out, hn)
			blob(&out, header)
			blob(&out, words(syms))
			blob(&out, words(extra))
		}
		if final == 1 {
			break
		}
	}
	ck(len(b)*8-r.p >= 32 && len(b)*8-r.p <= 39)
	out.WriteByte(2)
	u32(&out, len(b)*8-r.p)
	blob(&out, bitSlice(b, r.p, len(b)*8-r.p))
	ck(out.Len() <= 64<<20)
	return out.Bytes()
}
func selftest() {
	rng := rand.New(rand.NewSource(723))
	noise := make([]byte, 90000)
	rng.Read(noise)
	cases := [][]byte{{}, []byte("a"), bytes.Repeat([]byte("abcdeabcde--delta--"), 9000), noise}
	count := 0
	for _, level := range []int{flate.NoCompression, flate.BestSpeed, flate.DefaultCompression, flate.BestCompression, flate.HuffmanOnly} {
		for _, data := range cases {
			var b bytes.Buffer
			w, e := zlib.NewWriterLevel(&b, level)
			must(e)
			_, e = w.Write(data[:len(data)/2])
			must(e)
			must(w.Flush())
			_, e = w.Write(data[len(data)/2:])
			must(e)
			must(w.Close())
			p := packZ(b.Bytes())
			u := unpackZ(p)
			ck(bytes.Equal(u, b.Bytes()))
			zr, e := zlib.NewReader(bytes.NewReader(u))
			must(e)
			d, e := io.ReadAll(zr)
			must(e)
			must(zr.Close())
			ck(bytes.Equal(d, data))
			count++
		}
	}
	// Explicit fixed block with an empty stream; include nonzero final padding.
	fixed := []byte{0x78, 0x01, 0x03, 0xfc, 0, 0, 0, 1}
	ck(bytes.Equal(unpackZ(packZ(fixed)), fixed))
	count++
	fmt.Printf("{\"passed\":true,\"roundtrips\":%d}\n", count)
}
func unpackZInto(b []byte, dst []byte) []byte {
	r := input{b: b}
	ck(string(r.take(4)) == "ZT01")
	n := r.num()
	ck(n >= 6 && n <= 16<<20)
	hash := r.take(32)
	ck(len(dst) == n)
	w := bw{b: dst}
	w.copyBits(&br{b: r.take(2)}, 16)
	finalSeen := false
	for blocks := 0; ; blocks++ {
		ck(blocks <= 65536)
		typ := r.take(1)[0]
		hn := r.num()
		hb := r.blob()
		ck(len(hb) == (hn+7)/8)
		hr := br{b: hb}
		if typ == 2 {
			ck(finalSeen && hn >= 32 && hn <= 39)
			w.copyBits(&hr, hn)
			break
		}
		ck(!finalSeen)
		if typ == 0 { // Stored block remains a raw bit span; alignment is relative to the output.
			ck(hn >= 35)
			probe := br{b: hb}
			finalSeen = probe.get(1) == 1
			ck(probe.get(2) == 0)
			w.copyBits(&hr, hn)
		} else {
			ck(typ == 1)
			finalSeen = hr.get(1) == 1
			bt := hr.get(2)
			ll, dd := trees(&hr, bt, false)
			ck(hr.p == hn)
			w.copyBits(&br{b: hb}, hn)
			sb, eb := r.blob(), r.blob()
			ck(len(sb) >= 2 && len(sb)%2 == 0 && len(eb)%6 == 0)
			ep := 0
			for i := 0; i < len(sb); i += 2 {
				s := int(binary.LittleEndian.Uint16(sb[i:]))
				ll.write(&w, s)
				if s == 256 {
					ck(i == len(sb)-2)
					break
				}
				ck(i < len(sb)-2)
				if s > 256 {
					ck(s <= 285 && ep+6 <= len(eb))
					x := uint32(binary.LittleEndian.Uint16(eb[ep:]))
					d := int(binary.LittleEndian.Uint16(eb[ep+2:]))
					y := uint32(binary.LittleEndian.Uint16(eb[ep+4:]))
					ep += 6
					ck(d < 30)
					w.put(x, le[s-257])
					dd.write(&w, d)
					w.put(y, de[d])
				}
			}
			ck(ep == len(eb))
		}
	}
	ck(r.p == len(b) && w.p == n*8)
	w.finish()
	s := sha256.Sum256(w.b)
	ck(bytes.Equal(s[:], hash))
	return w.b
}

func unpackZ(b []byte) []byte {
	r := input{b: b}
	ck(string(r.take(4)) == "ZT01")
	n := r.num()
	ck(n >= 6 && n <= 16<<20)
	return unpackZInto(b, make([]byte, n))
}
