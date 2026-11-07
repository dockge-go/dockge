package handler

import "testing"

func TestLogRecordTimestampAndOriginalText(t *testing.T) {
	for _, tc := range []struct{ input, text, stamp string }{
		{"app-1 | 2026-11-08T08:00:00.123456789Z 中文\n", "app-1 | 中文\n", "2026-11-08T08:00:00.123456789Z"},
		{"2026-11-08T08:00:00Z [error] 原始日志\n", "[error] 原始日志\n", "2026-11-08T08:00:00.000000000Z"},
		{"2026-11-08T16:00:00.123+08:00 ok\n", "ok\n", "2026-11-08T08:00:00.123000000Z"},
		{"runtime error without timestamp\n", "runtime error without timestamp\n", ""},
	} {
		got := parseLogRecord(tc.input)
		if got.Text != tc.text || got.Time != tc.stamp {
			t.Errorf("%q: %+v", tc.input, got)
		}
	}
}
