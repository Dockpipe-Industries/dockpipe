//go:build linux

package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/boundedcmd"
)

func TestMain(tests *testing.M) {
	// These fixture branches are compiled only into the test executable. The
	// shipped program has no environment-selected executable or admission seam.
	if mode := os.Getenv("IDENTITY_IMAGE_TEST_MODE"); mode != "" {
		os.Exit(imageFixture(mode))
	}
	os.Exit(tests.Run())
}

func imageFixture(mode string) int {
	if mode == "verify" {
		image, err := inheritExecutable()
		if err != nil {
			return 42
		}
		defer image.Close()
		_, _ = io.WriteString(os.Stdout, "trusted-image\n")
		return 0
	}
	if mode == "anchor" {
		_, _ = io.WriteString(os.Stdout, "ready\n")
		var trigger [1]byte
		if _, err := io.ReadFull(os.Stdin, trigger[:]); err != nil {
			return 43
		}
		image, err := sealRunningExecutable()
		if err != nil {
			return 44
		}
		defer image.Close()
		command, err := image.command(context.Background(), "identity-probe")
		if err != nil {
			return 45
		}
		command.Env = []string{"IDENTITY_IMAGE_TEST_MODE=verify"}
		output, failure := boundedcmd.Run(command, 1024)
		if failure != nil {
			return 46
		}
		_, _ = io.WriteString(os.Stdout, output)
		return 0
	}
	return 47
}

func TestSealedImageRejectsContentAndSizeChanges(t *testing.T) {
	image, err := sealRunningExecutable()
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	info, err := image.file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := image.file.WriteAt([]byte("replacement"), 0); err == nil {
		t.Fatal("sealed executable remained writable")
	}
	for _, size := range []int64{info.Size() - 1, info.Size() + 1} {
		if image.file.Truncate(size) == nil {
			t.Fatal("sealed executable size could be changed")
		}
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, image.file.Fd(), addSeals, 0x0010); errno == 0 {
		t.Fatal("sealed executable allowed changing its seals")
	}
	if _, err := image.command(context.Background(), "/tmp/caller-selected-program"); err == nil {
		t.Fatal("caller-selected executable role accepted")
	}
}

func TestSealedDescriptorSurvivesExplicitFD3Exec(t *testing.T) {
	image, err := sealRunningExecutable()
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command, err := image.command(ctx, "identity-probe")
	if err != nil {
		t.Fatal(err)
	}
	command.Env = []string{"IDENTITY_IMAGE_TEST_MODE=verify"}
	output, failure := boundedcmd.Run(command, 1024)
	if failure != nil || output != "trusted-image\n" {
		t.Fatalf("sealed FD3 execution failed: %v %q", failure, output)
	}
}

func TestInheritedImageRejectsUnsealedAndForeignBytes(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		var image *os.File
		var err error
		if foreign {
			image, err = newMemfd()
			if err == nil {
				_, err = image.Write([]byte("\x7fELFuntrusted-image"))
			}
			if err == nil {
				_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, image.Fd(), addSeals, requiredSeals)
				if errno != 0 {
					err = errno
				}
			}
		} else {
			image, err = os.Open("/proc/self/exe")
		}
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		command := exec.CommandContext(ctx, os.Args[0])
		command.Args[0] = "identity-probe"
		command.Env = []string{"IDENTITY_IMAGE_TEST_MODE=verify"}
		command.ExtraFiles = []*os.File{image}
		output, failure := boundedcmd.Run(command, 1024)
		cancel()
		_ = image.Close()
		if output != "" || failure == nil || failure.ExitCode == nil || *failure.ExitCode != 42 {
			t.Fatal("unsealed or mismatched inherited executable was admitted")
		}
	}
}

func TestPackageExecutableReplacementCannotSelectWorkerBytes(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "identity-cap-helper")
	source, err := os.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	copyFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyError := io.Copy(copyFile, io.LimitReader(source, maximumExecutableBytes+1))
	closeError := copyFile.Close()
	if copyError != nil || closeError != nil {
		t.Fatal("could not prepare executable replacement fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path)
	command.Env = []string{"IDENTITY_IMAGE_TEST_MODE=anchor"}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(output)
	if ready, err := reader.ReadString('\n'); err != nil || ready != "ready\n" {
		t.Fatal("replacement fixture never reached its running-inode barrier")
	}
	if err := os.Rename(path, path+".executing"); err != nil {
		t.Fatal(err)
	}
	// Replace the actual daemon path before it opens /proc/self/exe, and supply
	// malicious copies at both obsolete package execution paths as well.
	for _, name := range []string{"identity-cap-helper", "identity-worker", "identity-probe"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\necho substituted-program\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = io.WriteString(input, "G")
	_ = input.Close()
	result, err := io.ReadAll(io.LimitReader(reader, 1024))
	waitError := command.Wait()
	if err != nil || waitError != nil || string(result) != "trusted-image\n" || strings.Contains(string(result), "substituted") {
		t.Fatalf("mutable package replacement affected executable authority: %q %v", result, waitError)
	}
}
