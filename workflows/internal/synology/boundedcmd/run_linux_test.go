//go:build linux

package boundedcmd

import "testing"

func TestStdoutAndStderrShareOneBudget(t *testing.T) {
	output := &helperOutput{remaining: 4}
	if _, err := (helperWriter{output: output, retain: true}).Write([]byte("user")); err != nil {
		t.Fatal(err)
	}
	if _, err := (helperWriter{output: output}).Write([]byte("secret")); err == nil || !output.exhausted {
		t.Fatal("stderr did not share the aggregate process output budget")
	}
	if output.stdout.String() != "user" {
		t.Fatal("stderr was retained")
	}
}
