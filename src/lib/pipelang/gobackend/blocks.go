package gobackend

import (
	"dockpipe/src/lib/pipelang/coreir"
	"fmt"
	"strings"
)

func (g *generator) emitGeneralBlock(out *strings.Builder, b *coreir.Block, parameters []coreir.Parameter, optional string) error {
	scope := append([]coreir.Parameter{}, parameters...)
	for _, s := range b.Statements {
		var value string
		if s.Value != nil {
			v, err := g.emitExpr(*s.Value, scope, optional)
			if err != nil {
				return err
			}
			value = v
		}
		switch s.Kind {
		case "local":
			if s.Value == nil {
				typ, err := g.goType(s.Local.Type, optional)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "var p%d %s; _ = p%d\n", s.Local.Position, typ, s.Local.Position)
			} else {
				fmt.Fprintf(out, "p%d := %s; _ = p%d\n", s.Local.Position, value, s.Local.Position)
			}
			scope = append(scope, *s.Local)
		case "assign":
			fmt.Fprintf(out, "p%d = %s\n", *s.Target, value)
		case "return":
			fmt.Fprintf(out, "return %s\n", value)
		case "if":
			fmt.Fprintf(out, "if %s {\n", value)
			if err := g.emitGeneralBlock(out, s.Then, scope, optional); err != nil {
				return err
			}
			if s.Else != nil {
				out.WriteString("} else {\n")
				if err := g.emitGeneralBlock(out, s.Else, scope, optional); err != nil {
					return err
				}
			}
			out.WriteString("}\n")
		case "block":
			out.WriteString("{\n")
			if err := g.emitGeneralBlock(out, s.Then, scope, optional); err != nil {
				return err
			}
			out.WriteString("}\n")
		default:
			return fmt.Errorf("invalid statement")
		}
	}
	return nil
}
