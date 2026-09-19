package repository

import "testing"

// TestIsUnderMountPoint 守护栈目录持久化判定：跳过根挂载点、支持嵌套与转义，
// 且无法判断时不误报（否则用户会看到假的「数据会丢」告警）。
func TestIsUnderMountPoint(t *testing.T) {
	onlyRoot := "36 35 0:31 / / rw,relatime - overlay overlay rw\n" +
		"37 36 0:32 / /proc rw,nosuid - proc proc rw\n"
	withStacksBind := onlyRoot + "40 36 0:40 / /opt/stacks rw,relatime - ext4 /dev/sdb rw\n"
	withDataVolume := onlyRoot + "41 36 0:41 / /app/data rw,relatime - ext4 /dev/sdc rw\n"
	withEscapedSpace := onlyRoot + `42 36 0:42 / /opt/my\040stacks rw - ext4 /dev/sdd rw` + "\n"

	cases := []struct {
		name      string
		mountinfo string
		dir       string
		want      bool
	}{
		{"无法读取时按已挂载处理", "", "/opt/stacks", true},
		{"绑定目录本身", withStacksBind, "/opt/stacks", true},
		{"绑定目录下的子路径", withStacksBind, "/opt/stacks/foo/bar", true},
		{"卷内子路径", withDataVolume, "/app/data/stacks", true},
		{"仅根挂载点不算持久化", onlyRoot, "/opt/stacks", false},
		{"仅根挂载点时深层路径同样不算", onlyRoot, "/var/lib/dockge/stacks", false},
		{"结尾斜杠归一化", withStacksBind, "/opt/stacks/", true},
		{"路径前缀相似但不同目录不算", withStacksBind, "/opt/stacks-other", false},
		{"挂载点含转义空格", withEscapedSpace, "/opt/my stacks", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnderMountPoint(tc.mountinfo, tc.dir); got != tc.want {
				t.Errorf("isUnderMountPoint(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}
