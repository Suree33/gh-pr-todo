package github

import (
	"encoding/json"
	"testing"
)

func TestPRMetaHeadRepositoryNameWithOwner(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
	}{
		{
			name: "uses nameWithOwner from gh pr view payload",
			payload: `{
				"headRefOid": "b93811e17d1cb86894fc3196f00be046d483b26e",
				"headRepository": {
					"id": "R_kgDOPgbooQ",
					"name": "gh-pr-todo",
					"nameWithOwner": "Suree33/gh-pr-todo"
				}
			}`,
			expected: "Suree33/gh-pr-todo",
		},
		{
			name: "falls back to owner and name",
			payload: `{
				"headRefOid": "b93811e17d1cb86894fc3196f00be046d483b26e",
				"headRepository": {
					"name": "gh-pr-todo",
					"owner": {
						"login": "Suree33"
					}
				}
			}`,
			expected: "Suree33/gh-pr-todo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta prMeta
			if err := json.Unmarshal([]byte(tt.payload), &meta); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if got := meta.headRepositoryNameWithOwner(); got != tt.expected {
				t.Fatalf("headRepositoryNameWithOwner() = %q, expected %q", got, tt.expected)
			}
		})
	}
}
