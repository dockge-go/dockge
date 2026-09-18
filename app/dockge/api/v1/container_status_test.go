package v1

import (
	"encoding/json"
	"testing"
)

func TestContainerStatusFrame_ContainsOnlyStatusFields(t *testing.T) {
	frame := ContainerStatusFrame{Containers: []ContainerStatusData{{
		ID:     "abc123",
		State:  "running",
		Status: "Up 10 seconds",
	}}}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"containers":[{"id":"abc123","state":"running","status":"Up 10 seconds"}]}`
	if string(data) != want {
		t.Fatalf("status frame = %s, want %s", data, want)
	}
}

// 锁定带计数与镜像列表的帧序列化：counts 与 images 同源同帧
//（前端原子写入快照后徽标计数与镜像列表永不脱节），
// 任一资源采集失败时整帧不推而非字段级省略。
func TestContainerStatusFrameWithCountsAndImages(t *testing.T) {
	frame := ContainerStatusFrame{
		Containers: []ContainerStatusData{{ID: "abc123", State: "running", Status: "Up"}},
		Counts: &ResourceCounts{
			ContainersTotal: 1, ContainersRunning: 1,
			StacksTotal: 2, StacksRunning: 1, ImagesTotal: 12,
		},
		Images: []DockerImageData{{ID: "img1", Repo: "nginx", Tag: "latest", SizeBytes: 1024, CreatedUnix: 1700000000}},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"containers":[{"id":"abc123","state":"running","status":"Up"}],` +
		`"counts":{"containersTotal":1,"containersRunning":1,"stacksTotal":2,"stacksRunning":1,"imagesTotal":12},` +
		`"images":[{"id":"img1","repo":"nginx","tag":"latest","sizeBytes":1024,"createdAt":1700000000}]}`
	if string(data) != want {
		t.Fatalf("counts frame = %s, want %s", data, want)
	}
}
