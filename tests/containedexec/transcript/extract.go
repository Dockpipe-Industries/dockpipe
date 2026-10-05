package main

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

func extract(binary, d string) {
	b, e := os.ReadFile(binary)
	must(e)
	ck(len(b) <= 16<<20)
	f, e := elf.NewFile(bytes.NewReader(b))
	must(e)
	defer f.Close()
	ck(f.Class == elf.ELFCLASS64 && f.Data == elf.ELFDATA2LSB)
	must(os.Mkdir(d, 0700))
	ss := []*elf.Section{}
	for _, s := range f.Sections {
		if s.Flags&elf.SHF_COMPRESSED != 0 && (s.Name == ".debug_line" || s.Name == ".debug_loclists" || s.Name == ".debug_rnglists") {
			ss = append(ss, s)
		}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Offset < ss[j].Offset })
	ck(len(ss) == 3)
	scaffold := bytes.Clone(b)
	tokens := []byte{}
	spans := []span{}
	for _, s := range ss {
		if s.Offset > uint64(len(b)) || s.FileSize < 24 || s.FileSize > uint64(len(b))-s.Offset {
			panic("compressed section escapes input")
		}
		if s.Offset > math.MaxInt || s.FileSize > math.MaxInt {
			panic("compressed section exceeds address space")
		}
		off := int(s.Offset) + 24
		end := int(s.Offset) + int(s.FileSize)
		z := b[off:end]
		t := packZ(z)
		ck(bytes.Equal(unpackZ(t), z))
		spans = append(spans, span{off, len(z), len(tokens), len(t)})
		tokens = append(tokens, t...)
		clear(scaffold[off:end])
	}
	sum := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
	must(os.WriteFile(filepath.Join(d, "scaffold"), scaffold, 0600))
	must(os.WriteFile(filepath.Join(d, "selected.tokens"), tokens, 0600))
	v, e := json.Marshal(map[string]any{"key": filepath.Base(d), "size": len(b), "sha256": sum(b), "scaffold_sha256": sum(scaffold), "tokens_sha256": sum(tokens), "token_bytes": len(tokens), "spans": spans})
	must(e)
	must(os.WriteFile(filepath.Join(d, "splice.json"), v, 0600))
}
