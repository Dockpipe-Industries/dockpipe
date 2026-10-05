package gobackend

import (
	"fmt"
	"go/scanner"
	"go/token"
	"path"
	"strconv"

	"dockpipe/src/lib/pipelang/coreir"
)

// FunctionBinding connects a Core identity to its allocated Go declaration.
type FunctionBinding struct {
	Identity coreir.SemanticIdentity
	Name     string
}

// Generated contains source and the entrypoint names allocated in that source.
type Generated struct {
	Source    []byte
	Functions []FunctionBinding
}

type generator struct {
	functionNames map[string]string
	recordNames   map[string]string
}

// Generate accepts Core IR only and returns deterministic, gofmt-formatted Go.
func Generate(program coreir.Program) ([]byte, error) {
	generated, err := GenerateWithNames(program)
	return generated.Source, err
}

// GenerateWithNames also returns identity-based bindings for host entrypoints.
// FunctionName alone cannot account for other declarations in the package.
func GenerateWithNames(program coreir.Program) (Generated, error) {
	g := &generator{}
	source, err := g.generate(program)
	if err != nil {
		return Generated{}, err
	}
	result := Generated{Source: source}
	for _, function := range sortedFunctions(program.Functions) {
		result.Functions = append(result.Functions, FunctionBinding{
			Identity: function.Identity, Name: g.functionNames[identityKey(function.Identity)],
		})
	}
	return result, nil
}

// Reserve all preferred names first so a collision suffix cannot steal a
// noncolliding declaration's existing name. Input order is semantic identity order.
func allocateNames(keys, bases []string, used map[string]bool) map[string]string {
	reserved := make(map[string]bool, len(bases))
	for _, base := range bases {
		reserved[base] = true
	}
	allocated := make(map[string]string, len(keys))
	for index, key := range keys {
		base := bases[index]
		name := base
		if used[name] {
			for suffix := 2; ; suffix++ {
				name = base + "__" + strconv.Itoa(suffix)
				if !used[name] && !reserved[name] {
					break
				}
			}
		}
		used[name] = true
		allocated[key] = name
	}
	return allocated
}

func (g *generator) allocateFunctions(functions []coreir.Function, used map[string]bool) {
	var keys, bases []string
	for _, function := range functions {
		keys = append(keys, identityKey(function.Identity))
		bases = append(bases, FunctionName(function))
	}
	g.functionNames = allocateNames(keys, bases, used)
}

func (g *generator) allocateRecords(records []coreir.Type) {
	var keys, bases []string
	for _, record := range records {
		keys = append(keys, identityKey(*record.Identity))
		bases = append(bases, "PipeLangRecord"+exportedIdentifier(record.Identity.PackageID+"_"+record.Identity.Path))
	}
	g.recordNames = allocateNames(keys, bases, map[string]bool{})
}

// Inspect the actual emitted support rather than maintaining a second list of
// runtime names. Methods belong to their receiver, not the package namespace.
// The same check on the final file catches duplicate support declarations too.
func packageNames(source []byte) (map[string]bool, error) {
	// Tokenize target output only. This keeps the backend free of parser/AST
	// dependencies, including the Go AST used by the legacy source parser.
	var scan scanner.Scanner
	var scanErr error
	scan.Init(token.NewFileSet().AddFile("generated.go", -1, len(source)), source,
		func(_ token.Position, message string) {
			if scanErr == nil {
				scanErr = fmt.Errorf("scan generated declarations: %s", message)
			}
		}, 0)
	type item struct {
		kind token.Token
		text string
	}
	var tokens []item
	for {
		_, kind, text := scan.Scan()
		tokens = append(tokens, item{kind, text})
		if kind == token.EOF {
			break
		}
	}
	if scanErr != nil {
		return nil, scanErr
	}
	names := map[string]bool{}
	add := func(name string) error {
		if name == "_" {
			return nil
		}
		if names[name] {
			return fmt.Errorf("duplicate generated package name %q", name)
		}
		names[name] = true
		return nil
	}
	// Skip one declaration/specification, respecting nested types, signatures,
	// generic arguments, composite literals, and function bodies.
	skip := func(index int) int {
		depth := 0
		for index < len(tokens)-1 {
			kind := tokens[index].kind
			if depth == 0 && (kind == token.SEMICOLON || kind == token.RPAREN) {
				break
			}
			switch kind {
			case token.LPAREN, token.LBRACK, token.LBRACE:
				depth++
			case token.RPAREN, token.RBRACK, token.RBRACE:
				depth--
			}
			index++
		}
		return index
	}
	for index := 0; index < len(tokens)-1; {
		kind := tokens[index].kind
		index++
		if kind == token.SEMICOLON {
			continue
		}
		if kind == token.PACKAGE {
			index = skip(index)
			continue
		}
		if kind == token.FUNC {
			// A receiver starts with '('. Its method name is receiver-scoped.
			if tokens[index].kind == token.IDENT {
				if err := add(tokens[index].text); err != nil {
					return nil, err
				}
			}
			index = skip(index)
			continue
		}
		if kind != token.TYPE && kind != token.CONST && kind != token.VAR && kind != token.IMPORT {
			return nil, fmt.Errorf("unexpected generated package token %s", kind)
		}
		grouped := tokens[index].kind == token.LPAREN
		if grouped {
			index++
		}
		for {
			for tokens[index].kind == token.SEMICOLON {
				index++
			}
			if grouped && tokens[index].kind == token.RPAREN {
				index++
				break
			}
			if kind == token.IMPORT {
				name := tokens[index].text
				if tokens[index].kind == token.STRING {
					imported, err := strconv.Unquote(name)
					if err != nil {
						return nil, err
					}
					name = path.Base(imported)
				}
				if err := add(name); err != nil {
					return nil, err
				}
			} else {
				for {
					if tokens[index].kind != token.IDENT {
						return nil, fmt.Errorf("generated declaration lacks a name")
					}
					if err := add(tokens[index].text); err != nil {
						return nil, err
					}
					index++
					if (kind != token.CONST && kind != token.VAR) || tokens[index].kind != token.COMMA {
						break
					}
					index++
				}
			}
			index = skip(index)
			if !grouped {
				break
			}
			if tokens[index].kind == token.EOF {
				return nil, fmt.Errorf("unterminated generated declaration group")
			}
		}
	}
	return names, nil
}
