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
