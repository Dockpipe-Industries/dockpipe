package pipelang

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokInt
	tokFloat
	tokString
	tokBool
	tokInterface
	tokClass
	tokStruct
	tokPublic
	tokPrivate
	tokLBrace
	tokRBrace
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokColon
	tokSemi
	tokComma
	tokAssign
	tokArrow
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokBang
	tokEQ
	tokNE
	tokLT
	tokLE
	tokGT
	tokGE
	tokAndAnd
	tokOrOr
)

type token struct {
	kind tokenKind
	lit  string
	span Span
}

func lex(sources *SourceSet, file *SourceFile) ([]token, error) {
	src := file.Text
	span := func(start, end int) Span { return Span{File: file.ID, Start: start, End: end} }
	out := make([]token, 0, 128)
	i := 0
	for i < len(src) {
		ch := src[i]
		if isSpace(ch) {
			i++
			continue
		}
		if ch == '/' && i+1 < len(src) && src[i+1] == '/' {
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if isIdentStart(ch) {
			start := i
			i++
			for i < len(src) && isIdentPart(src[i]) {
				i++
			}
			lit := src[start:i]
			switch lit {
			case "Interface":
				out = append(out, token{kind: tokInterface, lit: lit, span: span(start, i)})
			case "Class":
				out = append(out, token{kind: tokClass, lit: lit, span: span(start, i)})
			case "Struct":
				out = append(out, token{kind: tokStruct, lit: lit, span: span(start, i)})
			case "public":
				out = append(out, token{kind: tokPublic, lit: lit, span: span(start, i)})
			case "private":
				out = append(out, token{kind: tokPrivate, lit: lit, span: span(start, i)})
			case "true", "false":
				out = append(out, token{kind: tokBool, lit: lit, span: span(start, i)})
			default:
				out = append(out, token{kind: tokIdent, lit: lit, span: span(start, i)})
			}
			continue
		}
		if isDigit(ch) {
			start := i
			isFloat := false
			i++
			for i < len(src) && isDigit(src[i]) {
				i++
			}
			if i < len(src) && src[i] == '.' {
				isFloat = true
				i++
				if i >= len(src) || !isDigit(src[i]) {
					return nil, oneDiagnostic(sources, CodeInvalidNumber, CategoryLexical, span(start, i), "invalid float literal")
				}
				for i < len(src) && isDigit(src[i]) {
					i++
				}
			}
			lit := src[start:i]
			if isFloat {
				out = append(out, token{kind: tokFloat, lit: lit, span: span(start, i)})
			} else {
				out = append(out, token{kind: tokInt, lit: lit, span: span(start, i)})
			}
			continue
		}
		if ch == '"' {
			start := i
			i++
			var b strings.Builder
			for i < len(src) {
				if src[i] == '"' {
					i++
					out = append(out, token{kind: tokString, lit: b.String(), span: span(start, i)})
					goto next
				}
				if src[i] == '\\' {
					if i+1 >= len(src) {
						return nil, oneDiagnostic(sources, CodeUnterminatedText, CategoryLexical, span(start, len(src)), "unterminated escape sequence")
					}
					esc := src[i : i+2]
					u, err := strconv.Unquote("\"" + esc + "\"")
					if err != nil {
						return nil, oneDiagnostic(sources, CodeInvalidEscape, CategoryLexical, span(i, min(i+2, len(src))), "invalid escape sequence")
					}
					b.WriteString(u)
					i += 2
					continue
				}
				b.WriteByte(src[i])
				i++
			}
			return nil, oneDiagnostic(sources, CodeUnterminatedText, CategoryLexical, span(start, len(src)), "unterminated string literal")
		}

		switch ch {
		case '{':
			out = append(out, token{kind: tokLBrace, lit: "{", span: span(i, i+1)})
			i++
		case '}':
			out = append(out, token{kind: tokRBrace, lit: "}", span: span(i, i+1)})
			i++
		case '(':
			out = append(out, token{kind: tokLParen, lit: "(", span: span(i, i+1)})
			i++
		case ')':
			out = append(out, token{kind: tokRParen, lit: ")", span: span(i, i+1)})
			i++
		case '[':
			out = append(out, token{kind: tokLBracket, lit: "[", span: span(i, i+1)})
			i++
		case ']':
			out = append(out, token{kind: tokRBracket, lit: "]", span: span(i, i+1)})
			i++
		case ':':
			out = append(out, token{kind: tokColon, lit: ":", span: span(i, i+1)})
			i++
		case ';':
			out = append(out, token{kind: tokSemi, lit: ";", span: span(i, i+1)})
			i++
		case ',':
			out = append(out, token{kind: tokComma, lit: ",", span: span(i, i+1)})
			i++
		case '+':
			out = append(out, token{kind: tokPlus, lit: "+", span: span(i, i+1)})
			i++
		case '-':
			out = append(out, token{kind: tokMinus, lit: "-", span: span(i, i+1)})
			i++
		case '*':
			out = append(out, token{kind: tokStar, lit: "*", span: span(i, i+1)})
			i++
		case '/':
			out = append(out, token{kind: tokSlash, lit: "/", span: span(i, i+1)})
			i++
		case '=':
			if i+1 < len(src) && src[i+1] == '>' {
				out = append(out, token{kind: tokArrow, lit: "=>", span: span(i, i+2)})
				i += 2
			} else if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokEQ, lit: "==", span: span(i, i+2)})
				i += 2
			} else {
				out = append(out, token{kind: tokAssign, lit: "=", span: span(i, i+1)})
				i++
			}
		case '!':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokNE, lit: "!=", span: span(i, i+2)})
				i += 2
			} else {
				out = append(out, token{kind: tokBang, lit: "!", span: span(i, i+1)})
				i++
			}
		case '<':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokLE, lit: "<=", span: span(i, i+2)})
				i += 2
			} else {
				out = append(out, token{kind: tokLT, lit: "<", span: span(i, i+1)})
				i++
			}
		case '>':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{kind: tokGE, lit: ">=", span: span(i, i+2)})
				i += 2
			} else {
				out = append(out, token{kind: tokGT, lit: ">", span: span(i, i+1)})
				i++
			}
		case '&':
			if i+1 < len(src) && src[i+1] == '&' {
				out = append(out, token{kind: tokAndAnd, lit: "&&", span: span(i, i+2)})
				i += 2
			} else {
				return nil, oneDiagnostic(sources, CodeUnexpectedChar, CategoryLexical, span(i, i+1), "unexpected character '&'")
			}
		case '|':
			if i+1 < len(src) && src[i+1] == '|' {
				out = append(out, token{kind: tokOrOr, lit: "||", span: span(i, i+2)})
				i += 2
			} else {
				return nil, oneDiagnostic(sources, CodeUnexpectedChar, CategoryLexical, span(i, i+1), "unexpected character '|'")
			}
		default:
			return nil, oneDiagnostic(sources, CodeUnexpectedChar, CategoryLexical, span(i, i+1), fmt.Sprintf("unexpected character %q", ch))
		}
	next:
	}
	out = append(out, token{kind: tokEOF, span: span(len(src), len(src))})
	return out, nil
}

func isSpace(ch byte) bool { return unicode.IsSpace(rune(ch)) }
func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}
func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}
