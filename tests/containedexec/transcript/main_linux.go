package main

import (
	"bufio"
	"encoding/json"
	"os"
	"syscall"
	"time"
)

type span struct {
	Offset      int
	Size        int
	TokenOffset int
	TokenSize   int
}
type request struct {
	Input  string
	Output string
	Length int
	Spans  []span
}

func splice(q request) (elapsed float64, err string) {
	defer func() {
		if x := recover(); x != nil {
			err = "splice rejected"
		}
	}()
	t := time.Now()
	ck(q.Length > 0 && q.Length <= 16<<20 && len(q.Spans) > 0 && len(q.Spans) <= 256)
	f, e := os.Open(q.Input)
	must(e)
	defer f.Close()
	st, e := f.Stat()
	must(e)
	ck(st.Size() > 0 && st.Size() <= 64<<20)
	b, e := syscall.Mmap(int(f.Fd()), 0, int(st.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	must(e)
	defer syscall.Munmap(b)
	o, e := os.OpenFile(q.Output, os.O_RDWR, 0)
	must(e)
	defer o.Close()
	st, e = o.Stat()
	must(e)
	ck(st.Size() == int64(q.Length))
	out, e := syscall.Mmap(int(o.Fd()), 0, q.Length, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	must(e)
	defer syscall.Munmap(out)
	end, te := 0, 0
	for _, s := range q.Spans {
		ck(s.Offset >= end && s.Size > 0 && s.Offset <= len(out)-s.Size)
		ck(s.TokenOffset == te && s.TokenSize > 0 && s.TokenOffset <= len(b)-s.TokenSize)
		end = s.Offset + s.Size
		te = s.TokenOffset + s.TokenSize
		unpackZInto(b[s.TokenOffset:te], out[s.Offset:end])
	}
	ck(te == len(b))
	return time.Since(t).Seconds(), ""
}
func main() {
	ck(len(os.Args) >= 2)
	switch os.Args[1] {
	case "selftest":
		selftest()
	case "extract":
		ck(len(os.Args) == 4)
		extract(os.Args[2], os.Args[3])
	case "serve":
		in := bufio.NewScanner(os.Stdin)
		in.Buffer(make([]byte, 4096), 1<<20)
		out := json.NewEncoder(os.Stdout)
		for in.Scan() {
			var q request
			must(json.Unmarshal(in.Bytes(), &q))
			if q.Input == "" {
				must(out.Encode(map[string]any{"ready": true}))
				continue
			}
			t, e := splice(q)
			must(out.Encode(map[string]any{"seconds": t, "error": e}))
		}
		must(in.Err())
	default:
		panic("expected extract, serve or selftest")
	}
}
