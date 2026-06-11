package opcua

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

var (
	dataContractFromDefaultsRE = regexp.MustCompile(`msg\.meta\.data_contract\s*=\s*["']([^"']*)["']`)
	virtualPathFromDefaultsRE  = regexp.MustCompile(`msg\.meta\.virtual_path\s*=\s*["']([^"']*)["']`)
	nodeIDFromCaseRE           = regexp.MustCompile(`^case\s+(.+?):`)
)

var conditionFieldExpressions = map[string]string{
	"Node ID":              "msg.meta.opcua_attr_nodeid",
	"Location Path Suffix": "msg.meta.location_path_suffix",
	"Data Contract":        "msg.meta.data_contract",
	"Virtual Path":         "msg.meta.virtual_path",
	"Tag Name":             "msg.meta.tag_name",
}

func buildProcessorYAMLFromStructured(proc *v1.OpcUaProcessorStructuredConfig) (string, error) {
	if proc == nil {
		return defaultProcessorYAML(), nil
	}

	defaultCode := strings.TrimSpace(proc.GetDefaultsCode())
	if defaultCode == "" {
		defaultCode = buildDefaultProcessorCode(proc.GetDefaults(), proc.GetTagMappings())
	}
	conditionCode := strings.TrimSpace(proc.GetConditionsYaml())
	if conditionCode == "" {
		conditionCode = buildConditionsYAML(proc.GetConditions())
	}

	adv := strings.TrimSpace(proc.GetAdvancedProcessing())
	if adv == "" {
		adv = "return msg;"
	}
	return buildTagProcessorWrapper(defaultCode, conditionCode, adv), nil
}

func defaultProcessorYAML() string {
	return buildTagProcessorWrapper(buildDefaultProcessorCode(nil, nil), "[]", "return msg;")
}

func buildDefaultProcessorCode(defaults *v1.OpcUaProcessorDefaults, mappings []*v1.OpcUaTagMapping) string {
	dc := "_historian"
	vp := ""
	lp := "{{ .location_path }}"
	if defaults != nil {
		if v := strings.TrimSpace(defaults.GetDataContract()); v != "" {
			dc = v
		}
		if v := strings.TrimSpace(defaults.GetVirtualPath()); v != "" {
			vp = v
		}
		if v := strings.TrimSpace(defaults.GetLocationPath()); v != "" {
			lp = v
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("msg.meta.location_path = %s;\n", jsonString(lp)))
	b.WriteString(fmt.Sprintf("msg.meta.data_contract = %s;\n", jsonString(dc)))
	b.WriteString("msg.meta.tag_name = msg.meta.opcua_tag_name;\n")
	b.WriteString("msg.payload = msg.payload;\n")
	b.WriteString(fmt.Sprintf("msg.meta.virtual_path = %s;\n", jsonString(vp)))
	b.WriteString("msg.meta.timestamp_ms = msg.meta.opcua_source_timestamp;\n")
	b.WriteString("switch(msg.meta.opcua_attr_nodeid) {\n")
	for _, row := range mappings {
		nodeID := strings.TrimSpace(row.GetNodeId())
		if nodeID == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("  case %s:\n", jsonString(nodeID)))
		if suffix := strings.TrimSpace(row.GetLocationPathSuffix()); suffix != "" {
			b.WriteString(fmt.Sprintf("    msg.meta.location_path += %s;\n", jsonString("."+suffix)))
		}
		rowDC := strings.TrimSpace(row.GetDataContract())
		if rowDC != "" && rowDC != dc {
			b.WriteString(fmt.Sprintf("    msg.meta.data_contract = %s;\n", jsonString(rowDC)))
		}
		if v := strings.TrimSpace(row.GetVirtualPath()); v != "" {
			b.WriteString(fmt.Sprintf("    msg.meta.virtual_path = %s;\n", jsonString(v)))
		}
		if v := strings.TrimSpace(row.GetTagName()); v != "" {
			b.WriteString(fmt.Sprintf("    msg.meta.tag_name = %s;\n", jsonString(v)))
		}
		b.WriteString("    break;\n")
	}
	b.WriteString("  default:\n    break;\n}\nreturn msg;")
	return b.String()
}

func buildConditionsYAML(conditions []*v1.OpcUaProcessorCondition) string {
	if len(conditions) == 0 {
		return "[]"
	}
	var b strings.Builder
	for i, cond := range conditions {
		expr := strings.TrimSpace(cond.GetIfExpression())
		if expr == "" {
			expr = conditionExpression(cond)
		}
		action := strings.TrimSpace(cond.GetAction())
		if action == "" {
			action = "return msg;"
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- if: ")
		b.WriteString(expr)
		b.WriteString("\n  then: |\n")
		for _, line := range strings.Split(action, "\n") {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func conditionExpression(cond *v1.OpcUaProcessorCondition) string {
	clauses := cond.GetClauses()
	if len(clauses) == 0 {
		return "true"
	}
	joiner := " && "
	if cond.GetJoiner() == "OR" {
		joiner = " || "
	}
	parts := make([]string, 0, len(clauses))
	for _, c := range clauses {
		parts = append(parts, clauseExpression(c))
	}
	return strings.Join(parts, joiner)
}

func clauseExpression(c *v1.OpcUaConditionClause) string {
	fieldExpr := conditionFieldExpressions[c.GetField()]
	if fieldExpr == "" {
		fieldExpr = conditionFieldExpressions["Node ID"]
	}
	val := jsonString(c.GetValue())
	switch c.GetOperator() {
	case "not equals (!==)":
		return fmt.Sprintf("%s !== %s", fieldExpr, val)
	case "starts with":
		return fmt.Sprintf("String(%s).startsWith(%s)", fieldExpr, val)
	case "ends with":
		return fmt.Sprintf("String(%s).endsWith(%s)", fieldExpr, val)
	case "contains":
		return fmt.Sprintf("String(%s).includes(%s)", fieldExpr, val)
	case "greater than (>)":
		return fmt.Sprintf("%s > %s", fieldExpr, val)
	case "less than (<)":
		return fmt.Sprintf("%s < %s", fieldExpr, val)
	case "greater or equal (>=)":
		return fmt.Sprintf("%s >= %s", fieldExpr, val)
	case "less or equal (<=)":
		return fmt.Sprintf("%s <= %s", fieldExpr, val)
	default:
		return fmt.Sprintf("%s === %s", fieldExpr, val)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func buildTagProcessorWrapper(defaultCode, conditionCode, advancedProcessing string) string {
	cond := strings.TrimSpace(conditionCode)
	if cond == "" {
		cond = "[]"
	}
	adv := strings.TrimSpace(advancedProcessing)
	if adv == "" {
		adv = "return msg;"
	}
	var b strings.Builder
	b.WriteString("tag_processor:\n")
	if strings.Contains(adv, "\n") {
		b.WriteString("  advancedProcessing: |-\n")
		b.WriteString(indentBlock(adv, 4))
	} else {
		b.WriteString("  advancedProcessing: ")
		b.WriteString(adv)
		b.WriteString("\n")
	}
	b.WriteString("  conditions:\n")
	if cond == "[]" {
		b.WriteString("    []\n")
	} else {
		b.WriteString(indentBlock(cond, 4))
	}
	b.WriteString("  defaults: |-\n")
	b.WriteString(indentBlock(strings.TrimSpace(defaultCode), 4))
	return b.String()
}

func indentBlock(value string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	var b strings.Builder
	for _, line := range strings.Split(value, "\n") {
		b.WriteString(pad)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func parseProcessorStructured(processorYAML, bufferYAML string, templateVars map[string]string) (*v1.OpcUaReadFlowSection, v1.SectionParseStatus) {
	section := &v1.OpcUaReadFlowSection{
		DataType: v1.OpcUaBridgeDataType_TIME_SERIES,
		Processor: &v1.OpcUaProcessorStructuredConfig{},
		YamlInject: &v1.OpcUaYamlInjectConfig{
			RawYaml: strings.TrimSpace(bufferYAML),
		},
		RawProcessorYaml: processorYAML,
		RawBufferYaml:    bufferYAML,
	}

	processorYAML = strings.TrimSpace(processorYAML)
	if processorYAML == "" {
		return section, v1.SectionParseStatus_PARSE_FAILED
	}

	defaultCode := extractYAMLSection(processorYAML, "defaults", "conditions", "advancedProcessing")
	conditionCode := extractYAMLSection(processorYAML, "conditions", "defaults", "advancedProcessing")
	advancedCode := extractYAMLSection(processorYAML, "advancedProcessing", "defaults", "conditions")
	if defaultCode == "" {
		defaultCode = buildDefaultProcessorCode(nil, nil)
	}
	if conditionCode == "" {
		conditionCode = "[]"
	}

	proc := section.Processor
	proc.DefaultsCode = defaultCode
	proc.ConditionsYaml = conditionCode
	proc.Defaults = parseDefaultsFromCode(defaultCode)

	mappings := parseAddressMappingsFromTemplate(templateVars)
	if len(mappings) == 0 {
		mappings = parseTagMappingsFromDefaultCode(defaultCode)
	}
	proc.TagMappings = mappings
	proc.Conditions = parseConditionsFromYAML(conditionCode)
	if advancedCode == "" {
		advancedCode = "return msg;"
	}
	proc.AdvancedProcessing = advancedCode

	status := v1.SectionParseStatus_PARSE_PARTIAL
	if proc.Defaults != nil || len(proc.TagMappings) > 0 {
		status = v1.SectionParseStatus_PARSE_OK
	}
	return section, status
}

func parseDefaultsFromCode(code string) *v1.OpcUaProcessorDefaults {
	d := &v1.OpcUaProcessorDefaults{
		DataContract: "_historian",
		LocationPath: "{{ .location_path }}",
	}
	if m := dataContractFromDefaultsRE.FindStringSubmatch(code); len(m) > 1 {
		d.DataContract = m[1]
	}
	if m := virtualPathFromDefaultsRE.FindStringSubmatch(code); len(m) > 1 {
		d.VirtualPath = m[1]
	}
	if m := locationPathFromDefaultsRE.FindStringSubmatch(code); len(m) > 1 {
		d.LocationPath = m[1]
	}
	return d
}

func parseTagMappingsFromDefaultCode(code string) []*v1.OpcUaTagMapping {
	var mappings []*v1.OpcUaTagMapping
	lines := strings.Split(code, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "case ") {
			continue
		}
		m := nodeIDFromCaseRE.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		nodeID := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		tm := &v1.OpcUaTagMapping{NodeId: nodeID, DataContract: "_historian"}
		for j := i + 1; j < len(lines); j++ {
			inner := strings.TrimSpace(lines[j])
			if strings.HasPrefix(inner, "case ") || inner == "default:" {
				break
			}
			if strings.Contains(inner, "location_path +=") {
				if v := extractQuotedAssignment(inner); v != "" {
					tm.LocationPathSuffix = strings.TrimPrefix(v, ".")
				}
			}
			if strings.Contains(inner, "data_contract =") {
				if v := extractQuotedAssignment(inner); v != "" {
					tm.DataContract = v
				}
			}
			if strings.Contains(inner, "virtual_path =") {
				if v := extractQuotedAssignment(inner); v != "" {
					tm.VirtualPath = v
				}
			}
			if strings.Contains(inner, "tag_name =") && !strings.Contains(inner, "opcua_tag_name") {
				if v := extractQuotedAssignment(inner); v != "" {
					tm.TagName = v
				}
			}
		}
		mappings = append(mappings, tm)
	}
	return mappings
}

func extractQuotedAssignment(line string) string {
	if idx := strings.Index(line, `"`); idx >= 0 {
		rest := line[idx+1:]
		if end := strings.Index(rest, `"`); end >= 0 {
			return rest[:end]
		}
	}
	return ""
}

func parseConditionsFromYAML(conditionYAML string) []*v1.OpcUaProcessorCondition {
	text := strings.TrimSpace(conditionYAML)
	if text == "" || text == "[]" {
		return nil
	}
	entries := regexp.MustCompile(`(?m)\n(?=-\s+if:)`).Split(text, -1)
	var out []*v1.OpcUaProcessorCondition
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		ifMatch := regexp.MustCompile(`(?s)^-\s+if:\s*(.*?)(?:\n|$)`).FindStringSubmatch(entry)
		thenMatch := regexp.MustCompile(`(?s)\n\s*then:\s*\|[-]?\n([\s\S]*)$`).FindStringSubmatch(entry)
		if len(ifMatch) < 2 {
			continue
		}
		expr := strings.TrimSpace(ifMatch[1])
		action := "return msg;"
		if len(thenMatch) >= 2 {
			action = stripBlockIndent(thenMatch[1])
		}
		joiner := "AND"
		if strings.Contains(expr, " || ") {
			joiner = "OR"
		}
		out = append(out, &v1.OpcUaProcessorCondition{
			IfExpression: expr,
			Joiner:       joiner,
			Action:       action,
			Clauses: []*v1.OpcUaConditionClause{
				parseClauseExpression(expr),
			},
		})
	}
	return out
}

func parseClauseExpression(expression string) *v1.OpcUaConditionClause {
	expr := strings.TrimSpace(expression)
	for label, fieldExpr := range conditionFieldExpressions {
		if strings.HasPrefix(expr, fieldExpr) {
			c := &v1.OpcUaConditionClause{Field: label}
			if strings.Contains(expr, "!==") {
				c.Operator = "not equals (!==)"
			} else {
				c.Operator = "equals (===)"
			}
			if m := regexp.MustCompile(`["']([^"']*)["']\s*$`).FindStringSubmatch(expr); len(m) > 1 {
				c.Value = m[1]
			}
			return c
		}
	}
	return &v1.OpcUaConditionClause{Field: "Node ID", Operator: "equals (===)"}
}

func stripBlockIndent(value string) string {
	lines := strings.Split(strings.Trim(strings.ReplaceAll(value, "\r\n", "\n"), "\n"), "\n")
	if len(lines) == 0 {
		return ""
	}
	minIndent := len(lines[0])
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if n < minIndent {
			minIndent = n
		}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= minIndent {
			line = line[minIndent:]
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return strings.TrimRight(strings.Join(out, "\n"), " \t")
}

func extractYAMLSection(yamlText, key string, nextKeys ...string) string {
	text := stripTagProcessorRoot(yamlText)
	lines := strings.Split(text, "\n")
	keyPrefix := key + ":"
	var start int = -1
	isBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, keyPrefix) {
			continue
		}
		start = i
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, keyPrefix))
		if rest == "|" || rest == "|-" {
			isBlock = true
		} else if rest != "" {
			return stripYAMLScalar(rest)
		}
		break
	}
	if start < 0 {
		return ""
	}
	if !isBlock {
		return ""
	}

	var body []string
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			body = append(body, line)
			continue
		}
		if isTopLevelYAMLKey(line, nextKeys...) {
			break
		}
		if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") {
			break
		}
		body = append(body, line)
	}
	return stripBlockIndent(strings.Join(body, "\n"))
}

func isTopLevelYAMLKey(line string, keys ...string) bool {
	trimmed := strings.TrimSpace(line)
	for _, key := range keys {
		if strings.HasPrefix(trimmed, key+":") {
			return true
		}
	}
	return false
}

func stripTagProcessorRoot(yaml string) string {
	text := strings.TrimSpace(strings.ReplaceAll(yaml, "\r\n", "\n"))
	if !strings.HasPrefix(text, "tag_processor:") {
		return text
	}
	return stripBlockIndent(strings.Join(strings.Split(text, "\n")[1:], "\n"))
}
