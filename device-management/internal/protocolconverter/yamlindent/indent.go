package yamlindent

import "strings"

// Leading returns the number of leading space/tab characters on a line.
func Leading(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// BlockKeyIndent returns the indent of the first key line inside a YAML block
// (e.g. lines under "modbus:" or "opcua:"). Defaults to 2 when not found.
func BlockKeyIndent(lines []string, blockMarker string) int {
	marker := strings.TrimSpace(blockMarker)
	if !strings.HasSuffix(marker, ":") {
		marker += ":"
	}
	seen := false
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == marker {
			seen = true
			continue
		}
		if !seen || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			continue
		}
		sep := strings.Index(trimmed, ":")
		if sep < 0 {
			continue
		}
		indent := Leading(line)
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		return 2
	}
	return minIndent
}

// ListItemsAfterKey reads "- item" lines following a key at keyLineIdx/keyIndent.
func ListItemsAfterKey(lines []string, keyLineIdx, keyIndent int) ([]string, int) {
	var items []string
	listIndent := -1
	last := keyLineIdx
	for i := keyLineIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := Leading(line)
		if indent <= keyIndent && strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if !strings.HasPrefix(trimmed, "- ") {
			if indent > keyIndent {
				break
			}
			continue
		}
		if listIndent < 0 {
			listIndent = indent
		}
		if indent < listIndent {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if item != "" {
			items = append(items, item)
		}
		last = i
	}
	return items, last
}

// IsChildKeyLine reports whether line is a sibling key at keyIndent under the same block.
func IsChildKeyLine(line string, keyIndent int) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") {
		return false
	}
	if strings.Index(trimmed, ":") < 0 {
		return false
	}
	return Leading(line) == keyIndent
}
