package workflowdocument

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// These are editor capacity limits, not workflow execution limits. They bound
// synchronous document transformations and the GUI's retained document graph.
const WorkflowDocumentMaxBytes = 1 << 20
const workflowDocumentMaxNodes = 8192
const workflowDocumentMaxDepth = 64
const workflowDocumentMaxEdits = 128

type WorkflowDocumentEdit struct {
	Operation   string          `json:"operation"`
	Path        []string        `json:"path"`
	Value       json.RawMessage `json:"value,omitempty"`
	Destination int             `json:"destination,omitempty"`
}

type WorkflowDocumentRequest struct {
	Source string                 `json:"source"`
	Edits  []WorkflowDocumentEdit `json:"edits,omitempty"`
}

type WorkflowDocumentResult struct {
	Source   string `json:"source"`
	Document any    `json:"document"`
}

// EditWorkflowDocument changes a syntax tree without executing workflows or
// reading their imports. Unedited fields and comments remain in the YAML tree.
func EditWorkflowDocument(request WorkflowDocumentRequest) (*WorkflowDocumentResult, error) {
	if len(request.Source) > WorkflowDocumentMaxBytes || len(request.Edits) > workflowDocumentMaxEdits {
		return nil, fmt.Errorf("visual editor capacity exceeded (1 MiB source, 128 edits per request); use the YAML editor")
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(request.Source))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("read workflow document: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("the visual editor requires one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("workflow must be a YAML mapping")
	}
	root := document.Content[0]
	if err := checkDocumentTree(root); err != nil {
		return nil, err
	}
	for _, edit := range request.Edits {
		if err := editDocumentNode(root, edit); err != nil {
			return nil, err
		}
		if err := checkDocumentTree(root); err != nil {
			return nil, err
		}
	}
	var value any
	if err := root.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	source := request.Source
	if len(request.Edits) != 0 {
		var output bytes.Buffer
		encoder := yaml.NewEncoder(&output)
		encoder.SetIndent(2)
		if err := encoder.Encode(&document); err != nil {
			return nil, err
		}
		if err := encoder.Close(); err != nil {
			return nil, err
		}
		if output.Len() > WorkflowDocumentMaxBytes {
			return nil, fmt.Errorf("edited workflow exceeds 1 MiB editor capacity")
		}
		source = output.String()
	}
	return &WorkflowDocumentResult{Source: source, Document: value}, nil
}

func checkDocumentTree(root *yaml.Node) error {
	type entry struct {
		node  *yaml.Node
		depth int
	}
	pending := []entry{{root, 0}}
	count := 0
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		count++
		if count > workflowDocumentMaxNodes || current.depth > workflowDocumentMaxDepth {
			return fmt.Errorf("visual editor capacity exceeded (8192 YAML nodes, 64 levels); use the YAML editor")
		}
		node := current.node
		if node.Kind == yaml.AliasNode || node.Anchor != "" {
			return fmt.Errorf("YAML anchors and aliases require the YAML editor; the source is unchanged")
		}
		if node.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(node.Content); i += 2 {
				key := node.Content[i]
				if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
					return fmt.Errorf("visual editor requires unique string mapping keys")
				}
				seen[key.Value] = true
			}
		}
		for _, child := range node.Content {
			pending = append(pending, entry{child, current.depth + 1})
		}
	}
	return nil
}

func documentChild(parent *yaml.Node, key string) (*yaml.Node, int, error) {
	switch parent.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(parent.Content); i += 2 {
			if parent.Content[i].Value == key {
				return parent.Content[i+1], i + 1, nil
			}
		}
		return nil, -1, nil
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(parent.Content) {
			return nil, -1, fmt.Errorf("invalid sequence index %q", key)
		}
		return parent.Content[index], index, nil
	default:
		return nil, -1, fmt.Errorf("document path crosses a scalar")
	}
}

func editDocumentNode(root *yaml.Node, edit WorkflowDocumentEdit) error {
	if len(edit.Path) == 0 || len(edit.Path) > workflowDocumentMaxDepth {
		return fmt.Errorf("edit requires a bounded non-root path")
	}
	parent := root
	for _, key := range edit.Path[:len(edit.Path)-1] {
		child, _, err := documentChild(parent, key)
		if err != nil {
			return err
		}
		if child == nil {
			return fmt.Errorf("document path %q does not exist", key)
		}
		parent = child
	}
	key := edit.Path[len(edit.Path)-1]
	old, index, err := documentChild(parent, key)
	if err != nil {
		return err
	}
	switch edit.Operation {
	case "set", "append":
		if !json.Valid(edit.Value) {
			return fmt.Errorf("edit value must be JSON")
		}
		var replacement yaml.Node
		if err := yaml.Unmarshal(edit.Value, &replacement); err != nil {
			return err
		}
		node := replacement.Content[0]
		// New values should read as YAML, not retain JSON flow/quote styling.
		pending := []*yaml.Node{node}
		for len(pending) > 0 {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			current.Style = 0
			if current.Kind == yaml.ScalarNode && strings.Contains(current.Value, "\n") {
				current.Style = yaml.LiteralStyle
			}
			pending = append(pending, current.Content...)
		}
		if edit.Operation == "append" {
			if old == nil || old.Kind != yaml.SequenceNode {
				return fmt.Errorf("append requires a sequence")
			}
			old.Content = append(old.Content, node)
		} else if old != nil {
			node.HeadComment, node.LineComment, node.FootComment = old.HeadComment, old.LineComment, old.FootComment
			parent.Content[index] = node
		} else if parent.Kind == yaml.MappingNode {
			parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node)
		} else {
			return fmt.Errorf("set requires an existing sequence item or mapping key")
		}
	case "remove":
		if old == nil {
			return nil
		}
		if parent.Kind == yaml.MappingNode {
			parent.Content = append(parent.Content[:index-1], parent.Content[index+1:]...)
		} else {
			parent.Content = append(parent.Content[:index], parent.Content[index+1:]...)
		}
	case "move":
		target := edit.Destination
		if parent.Kind != yaml.SequenceNode || target < 0 || target >= len(parent.Content) {
			return fmt.Errorf("move requires an index in the same sequence")
		}
		parent.Content = append(parent.Content[:index], parent.Content[index+1:]...)
		parent.Content = append(parent.Content, nil)
		copy(parent.Content[target+1:], parent.Content[target:])
		parent.Content[target] = old
	default:
		return fmt.Errorf("unsupported document edit %q", edit.Operation)
	}
	return nil
}
