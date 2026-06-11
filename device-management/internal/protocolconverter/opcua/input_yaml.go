package opcua

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

var hiddenInputYAMLKeys = map[string]struct{}{
	"endpoint": {}, "subscribeEnabled": {}, "useHeartbeat": {},
}

// advancedYAMLToField maps benthos-umh opcua_plugin YAML keys to proto fields.
var advancedYAMLToField = map[string]string{
	"profile":                      "profile",
	"serverProfile":                "profile", // legacy UI key
	"securityMode":                 "security_mode",
	"securityPolicy":               "security_policy",
	"clientCertificate":            "client_certificate_base64",
	"serverCertificateFingerprint": "server_certificate_fingerprint",
	"userCertificate":              "user_certificate_base64",
	"userPrivateKey":               "user_private_key_base64",
	"sessionTimeout":               "session_timeout_ms",
	"reconnectIntervalInSeconds":   "reconnect_interval_seconds",
}

func buildInputYAMLFromStructured(cfg *v1.OpcUaInputStructuredConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("input.structured is required")
	}
	std := cfg.GetStandard()
	if std == nil {
		std = &v1.OpcUaStandardInputConfig{}
	}
	adv := cfg.GetAdvanced()
	if adv == nil {
		adv = &v1.OpcUaAdvancedInputConfig{}
	}

	nodeIDs := collectNodeIDs(std)
	if len(nodeIDs) == 0 {
		return "", fmt.Errorf("input.structured.standard.subscribe_node_ids or node_ids_text must not be empty")
	}

	endpoint := strings.TrimSpace(std.GetEndpoint())
	if endpoint == "" {
		endpoint = "opc.tcp://{{ .IP }}:{{ .PORT }}"
	}

	lines := []string{"opcua:"}
	lines = append(lines, fmt.Sprintf("  endpoint: %s", formatYAMLScalar(endpoint)))
	lines = append(lines, fmt.Sprintf("  username: %s", formatYAMLScalar(adv.GetUsername())))
	lines = append(lines, fmt.Sprintf("  password: %s", formatYAMLScalar(adv.GetPassword())))

	subscribeEnabled := "true"
	if std != nil && !std.GetSubscribeEnabled() {
		subscribeEnabled = "false"
	}
	useHeartbeat := "true"
	if std != nil && !std.GetUseHeartbeat() {
		useHeartbeat = "false"
	}
	lines = append(lines, fmt.Sprintf("  subscribeEnabled: %s", subscribeEnabled))
	lines = append(lines, fmt.Sprintf("  useHeartbeat: %s", useHeartbeat))

	if std.GetPollRate() > 0 {
		lines = append(lines, fmt.Sprintf("  pollRate: %d", std.GetPollRate()))
	}

	lines = append(lines, "  nodeIDs:")
	for _, id := range nodeIDs {
		lines = append(lines, fmt.Sprintf("    - %s", formatYAMLScalar(id)))
	}

	appendAdvancedInputField(&lines, "profile", adv.GetProfile())
	appendAdvancedInputField(&lines, "securityMode", adv.GetSecurityMode())
	appendAdvancedInputField(&lines, "securityPolicy", adv.GetSecurityPolicy())
	appendAdvancedInputField(&lines, "clientCertificate", adv.GetClientCertificateBase64())
	appendAdvancedInputField(&lines, "serverCertificateFingerprint", adv.GetServerCertificateFingerprint())
	appendAdvancedInputField(&lines, "userCertificate", adv.GetUserCertificateBase64())
	appendAdvancedInputField(&lines, "userPrivateKey", adv.GetUserPrivateKeyBase64())

	if adv.GetSessionTimeoutMs() > 0 {
		lines = append(lines, fmt.Sprintf("  sessionTimeout: %d", adv.GetSessionTimeoutMs()))
	}
	if adv.GetInsecure() {
		lines = append(lines, "  insecure: true")
	}
	if adv.GetDirectConnect() {
		lines = append(lines, "  directConnect: true")
	}
	if adv.GetAutoReconnect() {
		lines = append(lines, "  autoReconnect: true")
	}
	if adv.GetReconnectIntervalSeconds() > 0 {
		lines = append(lines, fmt.Sprintf("  reconnectIntervalInSeconds: %d", adv.GetReconnectIntervalSeconds()))
	}
	if adv.GetQueueSize() > 0 {
		lines = append(lines, fmt.Sprintf("  queueSize: %d", adv.GetQueueSize()))
	}
	if adv.GetSamplingInterval() > 0 {
		lines = append(lines, fmt.Sprintf("  samplingInterval: %g", adv.GetSamplingInterval()))
	}

	for _, setting := range cfg.GetAdditionalSettings() {
		key := strings.TrimSpace(setting.GetKey())
		if key == "" {
			continue
		}
		appendYAMLSetting(&lines, key, setting.GetValue())
	}

	return strings.Join(lines, "\n"), nil
}

func collectNodeIDs(std *v1.OpcUaStandardInputConfig) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, n := range std.GetSubscribeNodeIds() {
		id := strings.TrimSpace(n.GetNodeId())
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range splitNodeIDLines(std.GetNodeIdsText()) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func splitNodeIDLines(text string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ',' }) {
		if id := strings.TrimSpace(part); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func appendAdvancedInputField(lines *[]string, yamlKey, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	appendYAMLSetting(lines, yamlKey, value)
}

func parseInputStructured(raw string) (*v1.OpcUaInputStructuredConfig, v1.SectionParseStatus) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, v1.SectionParseStatus_PARSE_FAILED
	}

	cfg := &v1.OpcUaInputStructuredConfig{
		Standard: &v1.OpcUaStandardInputConfig{
			SubscribeEnabled: true,
			UseHeartbeat:     true,
		},
		Advanced: &v1.OpcUaAdvancedInputConfig{},
	}
	std := cfg.Standard
	adv := cfg.Advanced

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	status := v1.SectionParseStatus_PARSE_OK

	for i := 0; i < len(lines); i++ {
		rawLine := lines[i]
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || trimmed == "opcua:" || strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(rawLine, "  ") {
			continue
		}
		sep := strings.Index(trimmed, ":")
		if sep < 0 {
			continue
		}
		yamlKey := strings.TrimSpace(trimmed[:sep])
		rawValue := strings.TrimSpace(trimmed[sep+1:])
		value := stripYAMLScalar(rawValue)

		if yamlKey == "nodeIDs" {
			ids := parseNodeIDsBlock(lines, i, rawValue)
			for _, id := range ids {
				std.SubscribeNodeIds = append(std.SubscribeNodeIds, &v1.OpcUaNodeSubscription{NodeId: id})
			}
			if len(ids) == 0 {
				status = v1.SectionParseStatus_PARSE_PARTIAL
			}
			continue
		}

		if rawValue == "|" || rawValue == "|-" {
			var block []string
			for i+1 < len(lines) && strings.HasPrefix(lines[i+1], "    ") {
				i++
				block = append(block, lines[i][4:])
			}
			value = strings.TrimRight(strings.Join(block, "\n"), " \t")
		}

		switch yamlKey {
		case "endpoint":
			std.Endpoint = value
		case "pollRate":
			if n, err := strconv.Atoi(value); err == nil {
				std.PollRate = int32(n)
			}
		case "subscribeInterval":
			// Legacy invalid key — ignored on read; devices may still return it.
		case "subscribeEnabled":
			std.SubscribeEnabled = value != "false"
		case "useHeartbeat":
			std.UseHeartbeat = value != "false"
		case "username":
			adv.Username = value
		case "password":
			adv.Password = value
		case "queueSize":
			if n, err := strconv.Atoi(value); err == nil {
				adv.QueueSize = int32(n)
			}
		case "samplingInterval":
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				adv.SamplingInterval = f
			}
		case "sessionTimeout":
			if n, err := strconv.Atoi(value); err == nil {
				adv.SessionTimeoutMs = int32(n)
			}
		case "insecure":
			adv.Insecure = value == "true"
		case "directConnect":
			adv.DirectConnect = value == "true"
		case "autoReconnect":
			adv.AutoReconnect = value == "true"
		case "reconnectIntervalInSeconds":
			if n, err := strconv.Atoi(value); err == nil {
				adv.ReconnectIntervalSeconds = int32(n)
			}
		default:
			if field, ok := advancedYAMLToField[yamlKey]; ok {
				setAdvancedField(adv, field, value)
				continue
			}
			if _, hidden := hiddenInputYAMLKeys[yamlKey]; hidden {
				continue
			}
			cfg.AdditionalSettings = append(cfg.AdditionalSettings, &v1.OpcUaAdditionalSetting{
				Key: yamlKey, Value: value,
			})
		}
	}

	if len(std.GetSubscribeNodeIds()) == 0 {
		if status == v1.SectionParseStatus_PARSE_OK {
			status = v1.SectionParseStatus_PARSE_PARTIAL
		}
	}
	return cfg, status
}

func parseNodeIDsBlock(lines []string, index int, rawValue string) []string {
	if rawValue != "" && rawValue != "[]" {
		return splitNodeIDLines(parseInlineNodeIDs(rawValue))
	}
	var ids []string
	for index+1 < len(lines) {
		next := lines[index+1]
		trimmed := strings.TrimSpace(next)
		if !strings.HasPrefix(next, "    ") || !strings.HasPrefix(trimmed, "- ") {
			break
		}
		index++
		if id := stripYAMLScalar(strings.TrimSpace(trimmed[2:])); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseInlineNodeIDs(value string) string {
	normalized := stripYAMLScalar(value)
	if normalized == "" || normalized == "[]" {
		return ""
	}
	if strings.HasPrefix(normalized, "[") && strings.HasSuffix(normalized, "]") {
		parts := strings.Split(normalized[1:len(normalized)-1], ",")
		var ids []string
		for _, p := range parts {
			if id := stripYAMLScalar(strings.TrimSpace(p)); id != "" {
				ids = append(ids, id)
			}
		}
		return strings.Join(ids, "\n")
	}
	return normalized
}

func setAdvancedField(adv *v1.OpcUaAdvancedInputConfig, field, value string) {
	switch field {
	case "profile":
		adv.Profile = value
	case "security_mode":
		adv.SecurityMode = value
	case "security_policy":
		adv.SecurityPolicy = value
	case "client_certificate_base64":
		adv.ClientCertificateBase64 = value
	case "server_certificate_fingerprint":
		adv.ServerCertificateFingerprint = value
	case "user_certificate_base64":
		adv.UserCertificateBase64 = value
	case "user_private_key_base64":
		adv.UserPrivateKeyBase64 = value
	case "session_timeout_ms":
		if n, err := strconv.Atoi(value); err == nil {
			adv.SessionTimeoutMs = int32(n)
		}
	case "reconnect_interval_seconds":
		if n, err := strconv.Atoi(value); err == nil {
			adv.ReconnectIntervalSeconds = int32(n)
		}
	}
}

func parseAddressMappingsFromTemplate(vars map[string]string) []*v1.OpcUaTagMapping {
	raw, ok := vars["AddressMappings"]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	var mappings []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &mappings); err != nil {
		return nil
	}
	out := make([]*v1.OpcUaTagMapping, 0, len(mappings))
	for _, m := range mappings {
		tm := &v1.OpcUaTagMapping{
			NodeId:             stringField(m, "", "Address", "nodeId", "node_id"),
			DataContract:       stringField(m, "_historian", "DataContract", "dataContract", "data_contract"),
			LocationPathSuffix: stringField(m, "", "LocationPathSuffix", "locationPathSuffix", "location_path_suffix"),
			TagName:            stringField(m, "", "TagName", "tagName", "tag_name"),
			VirtualPath:        stringField(m, "", "VirtualPath", "virtualPath", "virtual_path"),
		}
		if tm.NodeId != "" {
			out = append(out, tm)
		}
	}
	return out
}

func stringField(m map[string]interface{}, defaultVal string, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return defaultVal
}

func buildAddressMappingsJSON(mappings []*v1.OpcUaTagMapping) string {
	type row struct {
		Address            string `json:"Address"`
		DataContract       string `json:"DataContract"`
		LocationPathSuffix string `json:"LocationPathSuffix"`
		TagName            string `json:"TagName"`
		VirtualPath        string `json:"VirtualPath"`
	}
	rows := make([]row, 0, len(mappings))
	for _, m := range mappings {
		id := strings.TrimSpace(m.GetNodeId())
		if id == "" {
			continue
		}
		dc := strings.TrimSpace(m.GetDataContract())
		if dc == "" {
			dc = "_historian"
		}
		rows = append(rows, row{
			Address:            id,
			DataContract:       dc,
			LocationPathSuffix: strings.TrimSpace(m.GetLocationPathSuffix()),
			TagName:            strings.TrimSpace(m.GetTagName()),
			VirtualPath:        strings.TrimSpace(m.GetVirtualPath()),
		})
	}
	if len(rows) == 0 {
		return ""
	}
	b, _ := json.Marshal(rows)
	return string(b)
}

func stripYAMLScalar(value string) string {
	text := stripInlineYAMLComment(strings.TrimSpace(value))
	if len(text) >= 2 {
		if (text[0] == '"' && text[len(text)-1] == '"') || (text[0] == '\'' && text[len(text)-1] == '\'') {
			return text[1 : len(text)-1]
		}
	}
	return text
}

func stripInlineYAMLComment(value string) string {
	if idx := strings.Index(value, " #"); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func shouldQuoteYAMLScalar(value string) bool {
	text := value
	return text == "" ||
		text != strings.TrimSpace(text) ||
		strings.Contains(text, " #") ||
		strings.Contains(text, ": ") ||
		strings.Contains(text, "\n") ||
		strings.HasPrefix(text, "-") ||
		strings.HasPrefix(text, "{") ||
		strings.HasPrefix(text, "[")
}

func formatYAMLScalar(value string) string {
	text := value
	lower := strings.ToLower(text)
	if lower == "true" || lower == "false" {
		return lower
	}
	if shouldQuoteYAMLScalar(text) {
		b, _ := json.Marshal(text)
		return string(b)
	}
	return text
}

func appendYAMLSetting(lines *[]string, key, value string) {
	text := value
	if strings.Contains(text, "\n") {
		*lines = append(*lines, fmt.Sprintf("  %s: |", key))
		for _, line := range strings.Split(text, "\n") {
			*lines = append(*lines, "    "+line)
		}
		return
	}
	*lines = append(*lines, fmt.Sprintf("  %s: %s", key, formatYAMLScalar(text)))
}

var locationPathFromDefaultsRE = regexp.MustCompile(`msg\.meta\.location_path\s*=\s*["']([^"']*)["']`)
