// Package internal provides diff parsing utilities for extracting TODO comments.
package internal

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Suree33/gh-pr-todo/pkg/types"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var (
	todoRegex = regexp.MustCompile(`(?i)((?://|#|<!--|;|/\*)\s*(TODO|FIXME|HACK|NOTE|XXX|BUG):?\s*.*)`)
	hunkRegex = regexp.MustCompile(`^@@\s+\-\d+(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@`)

	commentNodeTypes = map[string]bool{
		"comment":               true,
		"line_comment":          true,
		"block_comment":         true,
		"documentation_comment": true,
	}
)

// lineRange represents a 1-based inclusive line range.
type lineRange struct {
	start int
	end   int
}

// fileChange holds diff metadata for a single file.
type fileChange struct {
	path        string
	addedRanges []lineRange
}

// ParseDiff extracts TODO comments from git diff output using regex (legacy).
func ParseDiff(diffOutput string) []types.TODO {
	var todos []types.TODO
	lines := strings.Split(diffOutput, "\n")

	var currentFile string
	var lineNumber int

	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			currentFile = path.Clean(after)
		} else if strings.HasPrefix(line, "@@") {
			if matches := hunkRegex.FindStringSubmatch(line); len(matches) > 1 {
				if startLine, err := strconv.Atoi(matches[1]); err == nil {
					lineNumber = startLine - 1
				}
			}
		} else if after, ok := strings.CutPrefix(line, "+"); ok {
			lineNumber++
			if matches := todoRegex.FindStringSubmatch(after); len(matches) > 2 {
				todos = append(todos, types.TODO{
					Filename: currentFile,
					Line:     lineNumber,
					Comment:  strings.TrimSpace(matches[1]),
					Type:     strings.ToUpper(matches[2]),
				})
			}
		} else if strings.HasPrefix(line, " ") {
			lineNumber++
		}
	}

	return todos
}

func ExtractChangedPaths(diffOutput string) []string {
	var paths []string
	seen := make(map[string]struct{})
	var inHunk bool
	for _, line := range strings.Split(diffOutput, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
		} else if strings.HasPrefix(line, "@@") {
			inHunk = true
		}
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok && !inHunk {
			p := path.Clean(after)
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// extractFileChanges parses unified diff output and returns per-file added line ranges.
func extractFileChanges(diffOutput string) []fileChange {
	var changes []fileChange
	lines := strings.Split(diffOutput, "\n")

	var current *fileChange
	var lineNumber int
	var inHunk bool

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
		} else if after, ok := strings.CutPrefix(line, "+++ b/"); ok && !inHunk {
			if current != nil {
				changes = append(changes, *current)
			}
			current = &fileChange{path: path.Clean(after)}
		} else if strings.HasPrefix(line, "@@") {
			if matches := hunkRegex.FindStringSubmatch(line); len(matches) > 1 {
				if startLine, err := strconv.Atoi(matches[1]); err == nil {
					lineNumber = startLine - 1
					inHunk = true
				}
			}
		} else if inHunk && strings.HasPrefix(line, "+") {
			lineNumber++
			if current != nil {
				n := len(current.addedRanges)
				if n > 0 && current.addedRanges[n-1].end == lineNumber-1 {
					current.addedRanges[n-1].end = lineNumber
				} else {
					current.addedRanges = append(current.addedRanges, lineRange{start: lineNumber, end: lineNumber})
				}
			}
		} else if inHunk && strings.HasPrefix(line, " ") {
			lineNumber++
		}
	}
	if current != nil {
		changes = append(changes, *current)
	}
	return changes
}

// ParseDiffWithContents extracts TODO comments using Tree-sitter for supported
// languages, falling back to regex for unsupported files.
// files maps file paths to their full post-change content. For files not present
// in the map, TODOs are extracted from the diff output using regex.
func ParseDiffWithContents(diffOutput string, files map[string][]byte) []types.TODO {
	changes := extractFileChanges(diffOutput)
	var todos []types.TODO
	var missingFiles []string

	for _, fc := range changes {
		if len(fc.addedRanges) == 0 {
			continue
		}

		content, ok := files[fc.path]
		if !ok {
			missingFiles = append(missingFiles, fc.path)
			continue
		}

		if found := parseTODOsWithTreeSitter(fc, content); found != nil {
			todos = append(todos, found...)
		} else {
			todos = append(todos, parseTODOsWithRegex(fc, content)...)
		}
	}

	if len(missingFiles) > 0 {
		missing := make(map[string]bool, len(missingFiles))
		for _, f := range missingFiles {
			missing[f] = true
		}
		for _, t := range ParseDiff(diffOutput) {
			if missing[t.Filename] {
				todos = append(todos, t)
			}
		}
	}

	return todos
}

// parseTODOsWithTreeSitter uses Tree-sitter to parse the file and extract TODO comments
// from comment nodes that intersect with added lines. Returns nil if the language
// is unsupported or parsing fails.
func parseTODOsWithTreeSitter(fc fileChange, content []byte) []types.TODO {
	entry := grammars.DetectLanguage(fc.path)
	if entry == nil {
		return nil
	}

	bt, err := grammars.ParseFile(fc.path, content)
	if err != nil {
		return nil
	}
	defer bt.Release()

	root := bt.RootNode()
	if root == nil {
		return nil
	}

	todos := make([]types.TODO, 0)
	walkTree(root, bt, fc, content, &todos)
	return todos
}

// walkTree recursively walks the AST and collects TODO comments from comment nodes.
func walkTree(node *gotreesitter.Node, bt *gotreesitter.BoundTree, fc fileChange, source []byte, todos *[]types.TODO) {
	nodeType := bt.NodeType(node)
	if isCommentNode(nodeType) {
		extractTODOsFromComment(node, bt, fc, source, todos)
		return
	}

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			walkTree(child, bt, fc, source, todos)
		}
	}
}

// isCommentNode returns true if the node type represents a comment.
func isCommentNode(nodeType string) bool {
	if commentNodeTypes[nodeType] {
		return true
	}
	return strings.Contains(nodeType, "comment")
}

// extractTODOsFromComment checks if a comment node intersects with added lines
// and extracts TODO markers from it.
func extractTODOsFromComment(node *gotreesitter.Node, bt *gotreesitter.BoundTree, fc fileChange, source []byte, todos *[]types.TODO) {
	// Tree-sitter rows are 0-based, our line ranges are 1-based
	nodeStartLine := int(node.StartPoint().Row) + 1

	commentText := bt.NodeText(node)
	lines := strings.Split(commentText, "\n")

	for i, line := range lines {
		fileLine := nodeStartLine + i
		if !lineInRanges(fileLine, fc.addedRanges) {
			continue
		}

		if matches := todoRegex.FindStringSubmatch(line); len(matches) > 2 {
			*todos = append(*todos, types.TODO{
				Filename: fc.path,
				Line:     fileLine,
				Comment:  strings.TrimSpace(matches[1]),
				Type:     strings.ToUpper(matches[2]),
			})
		}
	}
}

// lineInRanges returns true if line falls within any of the given ranges.
func lineInRanges(line int, ranges []lineRange) bool {
	for _, r := range ranges {
		if line >= r.start && line <= r.end {
			return true
		}
	}
	return false
}

// parseTODOsWithRegex is the fallback that applies regex matching against
// added lines identified from the file content.
func parseTODOsWithRegex(fc fileChange, content []byte) []types.TODO {
	var todos []types.TODO
	lines := strings.Split(string(content), "\n")

	for _, r := range fc.addedRanges {
		for line := r.start; line <= r.end && line <= len(lines); line++ {
			text := lines[line-1]
			if matches := todoRegex.FindStringSubmatch(text); len(matches) > 2 {
				todos = append(todos, types.TODO{
					Filename: fc.path,
					Line:     line,
					Comment:  strings.TrimSpace(matches[1]),
					Type:     strings.ToUpper(matches[2]),
				})
			}
		}
	}
	return todos
}
