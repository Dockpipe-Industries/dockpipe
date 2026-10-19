// Package mcpbundle inspects the binary/stdio MCPB subset without executing it.
package mcpbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Match the existing packagecatalog archive/expansion/member budgets. Metadata
// uses its 8 MiB budget; executable identification needs only the first 64 KiB.
const MaxArchiveBytes int64 = 512 << 20
const maxExpandedBytes uint64 = 2 << 30
const maxMembers = 100000
const maxMetadataBytes = 8 << 20

type Inspection struct {
	WorkspaceRoot       string   `json:"workspace_root,omitempty"`
	ConfigurationDigest string   `json:"configuration_sha256,omitempty"`
	Schema              string   `json:"schema"`
	SHA256              string   `json:"sha256"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Platform            string   `json:"platform"`
	Architecture        string   `json:"architecture"`
	EntryPoint          string   `json:"entry_point"`
	Arguments           []string `json:"arguments"`
	Files               int      `json:"files"`
	ExpandedBytes       uint64   `json:"expanded_bytes"`
	Executed            bool     `json:"executed"`
}

type manifest struct {
	ManifestVersion string `json:"manifest_version"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Description     string `json:"description"`
	Author          struct {
		Name string `json:"name"`
	} `json:"author"`
	Server        json.RawMessage            `json:"server"`
	UserConfig    map[string]json.RawMessage `json:"user_config"`
	Compatibility struct {
		Platforms []string                   `json:"platforms"`
		Runtimes  map[string]json.RawMessage `json:"runtimes"`
	} `json:"compatibility"`
}

type launch struct {
	Type       string `json:"type"`
	EntryPoint string `json:"entry_point"`
	Config     struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"mcp_config"`
}

// Inspect verifies all archive bytes and members before returning an inert launch
// description. It neither installs nor authorizes startup. Expected SHA is mandatory.
func Inspect(ctx context.Context, filename, expected, platform, arch string) (Inspection, error) {
	return InspectConfigured(ctx, filename, expected, platform, arch, Bindings{})
}

func InspectConfigured(ctx context.Context, filename, expected, platform, arch string, bindings Bindings) (Inspection, error) {
	var result Inspection
	digest, err := hex.DecodeString(expected)
	if err != nil || len(digest) != sha256.Size {
		return result, fmt.Errorf("expected SHA-256 must contain 64 hex digits")
	}
	file, err := os.Open(filename)
	if err != nil {
		return result, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > MaxArchiveBytes {
		return result, fmt.Errorf("MCPB must be a regular archive within 512 MiB")
	}
	hash := sha256.New()
	if _, err := copyContext(ctx, hash, file); err != nil {
		return result, err
	}
	if !bytes.Equal(hash.Sum(nil), digest) {
		return result, fmt.Errorf("MCPB checksum mismatch")
	}
	bundle, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return result, err
	}
	if len(bundle.File) > maxMembers {
		return result, fmt.Errorf("MCPB exceeds member budget")
	}
	members := map[string]*zip.File{}
	folded := map[string]*zip.File{}
	var total uint64
	for _, member := range bundle.File {
		name := strings.TrimSuffix(member.Name, "/")
		if !safePath(name) || folded[strings.ToLower(name)] != nil {
			return result, fmt.Errorf("unsafe or duplicate MCPB member")
		}
		folded[strings.ToLower(name)] = member
		mode := member.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return result, fmt.Errorf("MCPB links and special files are unsupported")
		}
		if mode.IsDir() && member.UncompressedSize64 != 0 {
			return result, fmt.Errorf("MCPB directory has content")
		}
		if member.UncompressedSize64 > maxExpandedBytes-total {
			return result, fmt.Errorf("MCPB exceeds 2 GiB expansion budget")
		}
		total += member.UncompressedSize64
		members[name] = member
	}
	// Reject file/directory conflicts before future materialization, including case aliases.
	for name := range members {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if entry := folded[strings.ToLower(parent)]; entry != nil && !entry.Mode().IsDir() {
				return result, fmt.Errorf("MCPB file shadows a directory")
			}
		}
	}
	metadata, ok := members["manifest.json"]
	if !ok || metadata.Mode().IsDir() || metadata.UncompressedSize64 > maxMetadataBytes {
		return result, fmt.Errorf("MCPB requires bounded manifest.json")
	}
	data, err := readMember(ctx, metadata, maxMetadataBytes)
	if err != nil {
		return result, err
	}
	var document manifest
	if err := json.Unmarshal(data, &document); err != nil {
		return result, err
	}
	var command launch
	decoder := json.NewDecoder(bytes.NewReader(document.Server))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		return result, fmt.Errorf("unsupported MCPB server contract: %w", err)
	}
	if document.ManifestVersion != "0.3" || document.Name == "" || document.Version == "" || document.Description == "" || document.Author.Name == "" {
		return result, fmt.Errorf("MCPB 0.3 identity and author are required")
	}
	if command.Type != "binary" || !safePath(command.EntryPoint) || command.Config.Command != "${__dirname}/"+command.EntryPoint {
		return result, fmt.Errorf("only contained MCPB binary entry points are supported")
	}
	if len(document.Compatibility.Runtimes) != 0 {
		return result, fmt.Errorf("MCPB runtime provisioning is not supported")
	}
	supported := false
	for _, candidate := range document.Compatibility.Platforms {
		if candidate == platform {
			supported = true
		}
	}
	if !supported {
		return result, fmt.Errorf("MCPB does not explicitly support selected platform")
	}
	arguments, workspaceRoot, configurationDigest, err := bindArguments(document.UserConfig, command.Config.Args, bindings)
	if err != nil {
		return result, err
	}
	executable := members[command.EntryPoint]
	if executable == nil || !executable.Mode().IsRegular() {
		return result, fmt.Errorf("MCPB entry point is missing or not regular")
	}
	header, err := prefixMember(ctx, executable, 64<<10)
	if err != nil {
		return result, err
	}
	if err := binaryTarget(header, platform, arch); err != nil {
		return result, err
	}
	// Reading every member also verifies ZIP CRCs; advertised expansion sizes alone
	// are not sufficient. Cancellation and actual byte budgets apply to the stream.
	for _, member := range bundle.File {
		if member.Mode().IsDir() {
			continue
		}
		stream, err := member.Open()
		if err != nil {
			return result, err
		}
		n, copyErr := copyContext(ctx, io.Discard, io.LimitReader(stream, int64(member.UncompressedSize64)+1))
		closeErr := stream.Close()
		if copyErr != nil {
			return result, copyErr
		}
		if closeErr != nil {
			return result, closeErr
		}
		if uint64(n) != member.UncompressedSize64 {
			return result, fmt.Errorf("MCPB member expansion differs from metadata")
		}
	}
	return Inspection{Schema: "dockpipe.mcpb.inspection/v1", SHA256: strings.ToLower(expected), Name: document.Name, Version: document.Version, Platform: platform, Architecture: arch, EntryPoint: command.EntryPoint, Arguments: arguments, Files: len(bundle.File), ExpandedBytes: total, WorkspaceRoot: workspaceRoot, ConfigurationDigest: configurationDigest}, nil
}

func safePath(name string) bool {
	if name == "" || name == "." || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00<>\"|?*") {
		return false
	}
	for _, character := range name {
		if character < 32 {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.TrimRight(part, ". ") != part {
			return false
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9') {
			return false
		}
	}
	return true
}
func copyContext(ctx context.Context, out io.Writer, in io.Reader) (int64, error) {
	buffer := make([]byte, 64<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := in.Read(buffer)
		if n > 0 {
			written, e := out.Write(buffer[:n])
			total += int64(written)
			if e != nil {
				return total, e
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}
func readMember(ctx context.Context, member *zip.File, limit int64) ([]byte, error) {
	stream, err := member.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var output bytes.Buffer
	_, err = copyContext(ctx, &output, io.LimitReader(stream, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(output.Len()) > limit {
		return nil, fmt.Errorf("MCPB member exceeds budget")
	}
	return output.Bytes(), nil
}
func prefixMember(ctx context.Context, member *zip.File, limit int64) ([]byte, error) {
	stream, err := member.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var output bytes.Buffer
	_, err = copyContext(ctx, &output, io.LimitReader(stream, limit))
	return output.Bytes(), err
}
func binaryTarget(header []byte, platform, arch string) error {
	match := false
	if len(header) >= 64 && arch != "" {
		switch platform {
		case "linux":
			if bytes.Equal(header[:4], []byte{0x7f, 'E', 'L', 'F'}) && header[4] == 2 && header[5] == 1 {
				machine := binary.LittleEndian.Uint16(header[18:20])
				match = (arch == "amd64" && machine == 62) || (arch == "arm64" && machine == 183)
			}
		case "darwin":
			if binary.LittleEndian.Uint32(header[:4]) == 0xfeedfacf {
				cpu := binary.LittleEndian.Uint32(header[4:8])
				match = (arch == "amd64" && cpu == 0x01000007) || (arch == "arm64" && cpu == 0x0100000c)
			}
		case "win32":
			offset := uint64(binary.LittleEndian.Uint32(header[60:64]))
			if string(header[:2]) == "MZ" && offset+6 <= uint64(len(header)) && string(header[offset:offset+4]) == "PE\x00\x00" {
				machine := binary.LittleEndian.Uint16(header[offset+4 : offset+6])
				match = (arch == "amd64" && machine == 0x8664) || (arch == "arm64" && machine == 0xaa64)
			}
		}
	}
	if !match {
		return fmt.Errorf("MCPB entry point does not match selected native platform/architecture")
	}
	return nil
}
