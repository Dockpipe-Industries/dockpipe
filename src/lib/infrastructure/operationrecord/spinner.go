package operationrecord

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// StartLineSpinner draws an indeterminate status line on w until stop is called.
// It is a no-op when w is not a terminal (e.g. CI, pipes).
func StartLineSpinner(w *os.File, message string) (stop func()) {
	if w == nil {
		w = os.Stderr
	}
	fd, ok := fdInt(w)
	if !ok || !isTerminalFn(fd) {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		chars := `|/-\`
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		clearWidth := len(message) + 8
		if clearWidth < 40 {
			clearWidth = 40
		}
		clear := strings.Repeat(" ", clearWidth)
		fmt.Fprintf(w, "\r  %s %c  ", message, chars[i%4])
		for {
			select {
			case <-done:
				fmt.Fprintf(w, "\r%s\r", clear)
				return
			case <-ticker.C:
				i++
				fmt.Fprintf(w, "\r  %s %c  ", message, chars[i%4])
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		wg.Wait()
	}
}

func fdInt(f *os.File) (int, bool) {
	fd := f.Fd()
	if fd > uintptr(math.MaxInt) {
		return 0, false
	}
	return int(fd), true
}

var isTerminalFn = term.IsTerminal
