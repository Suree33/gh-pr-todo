package types

import (
	"fmt"
	"strings"
)

type GroupBy string

const (
	GroupByNone GroupBy = ""
	GroupByFile GroupBy = "file"
	GroupByType GroupBy = "type"
)

func (g *GroupBy) Set(s string) error {
	switch strings.ToLower(s) {
	case string(GroupByFile):
		*g = GroupByFile
		return nil
	case string(GroupByType):
		*g = GroupByType
		return nil
	default:
		return fmt.Errorf("invalid value %q for --group-by (allowed: \"file\", \"type\")", s)
	}
}

func (g *GroupBy) String() string { return string(*g) }
func (g *GroupBy) Type() string   { return "group-by" }
