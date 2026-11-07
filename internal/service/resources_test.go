package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "dockge/api/v1"
)

func TestDuplicateVolumeReturnsConflictWithDetail(t *testing.T) {
	dir := t.TempDir()
	cli := `#!/bin/sh
case "$1 $2" in
  'ps -aq') exit 0 ;;
  'volume ls') printf 'existing\n' ;;
  'volume inspect') printf '[{"Name":"existing"}]\n' ;;
  *) echo "unexpected command: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo, _ := newTestAuthRepo(t)
	s := &ResourceService{repo: repo}
	_, err := s.Create(context.Background(), "volumes", v1.ResourceCreateRequest{Name: "existing"})
	if !errors.Is(err, v1.ErrConflict) || !strings.Contains(err.Error(), `卷 "existing" 已存在`) {
		t.Fatalf("资源冲突必须保留名称与具体原因：%v", err)
	}
}
