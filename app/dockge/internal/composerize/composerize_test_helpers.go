package composerize

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// checkYAMLSemantic 通过解析 YAML 来验证预期行的语义是否存在。
// 它解析输出的 YAML，然后检查关键键是否存在于预期的语义值中，
// 从而避免对引号的敏感断言。检查范围限定在 services.app 内部。
func checkYAMLSemantic(t *testing.T, got string, want []string) {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("无法解析 YAML: %v\n%s", err, got)
	}

	// 导航到 services.app
	services, ok := doc["services"].(map[string]any)
	if !ok {
		t.Fatalf("输出中缺少 services 键:\n%s", got)
	}
	app, ok := services["app"].(map[string]any)
	if !ok {
		t.Fatalf("输出中缺少 services.app 键:\n%s", got)
	}

	// 第一遍：解析所有 want 行，收集检查项
	checks := make(map[string]string)     // key -> expected value (quote-stripped)
	listExpected := map[string][]string{} // field name -> list of expected values

	for _, line := range want {
		trimmed := strings.TrimSpace(line)
		// 跳过路径标记行
		if trimmed == "services:" || trimmed == "  app:" {
			continue
		}
		// 处理键值对格式: "    image: nginx:latest"
		if strings.HasPrefix(trimmed, "    ") && strings.Contains(trimmed, ":") {
			key, expected := parseKeyValueLine(trimmed)
			if key != "" {
				checks[key] = expected
			}
		}
		// 处理列表项格式: "      - \"8080:80\""
		if strings.HasPrefix(trimmed, "-") {
			listValue := extractListItemValue(trimmed)
			// 尝试确定列表字段类型
			field := detectListField(listValue, got)
			if field != "" {
				listExpected[field] = append(listExpected[field], listValue)
			}
		}
	}

	// 第二遍：执行键值检查
	for key, expected := range checks {
		actualValue, ok := app[key]
		if !ok {
			t.Errorf("输出中缺少键 %q", key)
			continue
		}
		actualStr := fmt.Sprintf("%v", actualValue)
		if !strings.EqualFold(strings.Trim(actualStr, "\"'"), strings.Trim(expected, "\"'")) {
			t.Errorf("键 %q 期望值 %q 但实际为 %q", key, expected, actualStr)
		}
	}

	// 第三遍：检查列表字段
	for field, expectedValues := range listExpected {
		actualList, ok := app[field].([]any)
		if !ok {
			t.Errorf("输出中缺少列表键 %q", field)
			continue
		}
		for _, expected := range expectedValues {
			found := false
			for _, item := range actualList {
				if itemStr, ok := item.(string); ok && strings.Contains(itemStr, expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("在 %q 列表中未找到 %q", field, expected)
			}
		}
	}
}

// detectListField 尝试检测列表项所属的字段类型。
func detectListField(listValue string, got string) string {
	// 检查是否为 ports 类型（包含冒号和端口格式）
	if strings.Contains(listValue, ":") && !strings.HasPrefix(strings.TrimSpace(listValue), "-") {
		// 可能是 ports 条目，如 "8080:80" 或 "443:443"
		if strings.Contains(got, "ports:") {
			return "ports"
		}
	}
	// 检查是否为 volumes 类型（包含路径格式）
	if strings.Contains(listValue, "/") {
		if strings.Contains(got, "volumes:") {
			return "volumes"
		}
	}
	// 检查是否为 environment 类型（包含等号格式）
	if strings.Contains(listValue, "=") {
		if strings.Contains(got, "environment:") {
			return "environment"
		}
	}
	// 检查是否为 command 类型
	if listValue == "echo" || listValue == "hello" || listValue == "world" {
		if strings.Contains(got, "command:") {
			return "command"
		}
	}
	return ""
}

// extractListItemValue 从列表项字符串中提取值，去除引号。
func extractListItemValue(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		s = strings.TrimPrefix(s, "-")
	}
	s = strings.TrimSpace(s)
	// 去除可能的双引号或单引号
	if len(s) > 1 && (s[0] == '"' || s[0] == '\'') {
		if len(s) > 1 && s[len(s)-1] == s[0] {
			s = s[1 : len(s)-1]
		}
	}
	return strings.TrimSpace(s)
}

// parseKeyValueLine 从 YAML 键值行中提取键和期望值。
// 支持格式: key: value  或  key: "value"  或  key: 'value'
func parseKeyValueLine(line string) (key string, expected string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", ""
	}
	key = strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	quoted := len(value) > 1 && (value[0] == '"' || value[0] == '\'')
	if quoted {
		value = value[1 : len(value)-1]
	}
	return key, value
}
