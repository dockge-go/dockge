package handler

import (
	"regexp"
	"strings"
	"time"
)

// 运行时时间戳是断线恢复游标；剥离附加时间字段，不修改容器自己的日志文案。
var logTimestamp = regexp.MustCompile(`^(.*?)(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})) (.*)$`)

type logRecord struct {
	Text string `json:"text"`
	Time string `json:"time"`
}

func parseLogRecord(line string) logRecord {
	match := logTimestamp.FindStringSubmatch(strings.TrimSuffix(line, "\n"))
	if match == nil {
		return logRecord{Text: line}
	}
	stamp, err := time.Parse(time.RFC3339Nano, match[2])
	if err != nil {
		return logRecord{Text: line}
	}
	return logRecord{Text: match[1] + match[3] + "\n", Time: stamp.UTC().Format("2006-01-02T15:04:05.000000000Z")}
}
