package repository

import (
	"os"
	"path/filepath"
	"strings"
)

// mountinfoPath 是 Linux 挂载表；开发机（macOS/Windows）上不存在。
const mountinfoPath = "/proc/self/mountinfo"

// stacksPersistent 判断栈目录是否落在宿主挂载的卷/绑定目录上。
// 不在挂载点下意味着栈只写在容器可写层，容器重建即丢失——这是最容易造成
// 真实数据损失的部署疏忽，故由自检暴露给用户。
func stacksPersistent(dir string) bool {
	data, err := os.ReadFile(mountinfoPath)
	if err != nil {
		return true
	}
	return isUnderMountPoint(string(data), dir)
}

// isUnderMountPoint 判断 dir 是否等于或嵌套于某个挂载点。
// 容器根 "/" 恒为挂载点（overlay/rootfs），对它前缀匹配无意义，故跳过，
// 否则任何路径都会被判为已挂载、检测形同虚设。
// mountinfo 为空（无法读取）时返回 true：宁可漏报，不可误报。
func isUnderMountPoint(mountinfo, dir string) bool {
	if strings.TrimSpace(mountinfo) == "" {
		return true
	}
	dir = filepath.Clean(dir)
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		// 第 5 个字段为挂载点（经 \040 等八进制转义）
		if len(fields) < 5 {
			continue
		}
		mp := filepath.Clean(unescapeMountPoint(fields[4]))
		if mp == "/" || mp == "." {
			continue
		}
		if dir == mp || strings.HasPrefix(dir, mp+"/") {
			return true
		}
	}
	return false
}

// unescapeMountPoint 还原 mountinfo 的八进制转义（空格/制表/换行/反斜杠）。
func unescapeMountPoint(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}
