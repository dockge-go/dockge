// 镜像用例：列表、手动拉取与批量删除；引用合法性在此层校验（CLI 参数注入防线）。
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	v1 "dockge/api/v1"
	"dockge/internal/repository"

	"github.com/samber/do/v2"
)

// ImageService 提供镜像管理用例。
type ImageService interface {
	List(ctx context.Context) ([]v1.ImageData, error)
	PullStream(ctx context.Context, w io.Writer, ref string) error
	Delete(ctx context.Context, refs []string) (*v1.StackOpResponse, error)
}

// NewImageService 构造镜像服务，由注入容器调用。
func NewImageService(i do.Injector) (ImageService, error) {
	return &imageService{repo: do.MustInvoke[*repository.Repository](i)}, nil
}

type imageService struct {
	repo *repository.Repository
}

// List 返回本机镜像列表（含使用中标记）。
func (s *imageService) List(ctx context.Context) ([]v1.ImageData, error) {
	containers, err := s.repo.ResourceContainers(ctx)
	if err != nil {
		return nil, err
	}
	images, err := s.repo.ResourceImages(ctx, containers)
	if err != nil {
		return nil, err
	}
	list := make([]v1.ImageData, 0)
	for _, img := range images {
		refs := img.RepoTags
		if len(refs) == 0 {
			refs = []string{"<none>:<none>"}
		}
		for _, ref := range refs {
			repository, tag := ref, "<none>"
			if split := strings.LastIndex(ref, ":"); split > strings.LastIndex(ref, "/") {
				repository, tag = ref[:split], ref[split+1:]
			}
			list = append(list, v1.ImageData{ID: img.ID, Repository: repository, Tag: tag, Size: fmt.Sprintf("%d B", img.Size), CreatedSince: time.UnixMilli(img.Created).UTC().Format(time.RFC3339), InUse: img.InUse})
		}
	}
	return list, nil
}

// PullStream 流式拉取镜像，compose 输出逐行写入 w。
func (s *imageService) PullStream(ctx context.Context, w io.Writer, ref string) error {
	if !repository.ImageRefPattern.MatchString(ref) {
		return fmt.Errorf("%w: 非法镜像引用 %q（应形如 nginx:1.27 或 registry/repo:tag）", v1.ErrBadRequest, ref)
	}
	return s.repo.PullImageStream(ctx, w, ref)
}

// Delete 批量删除镜像，失败时带回 docker 输出。
func (s *imageService) Delete(ctx context.Context, refs []string) (*v1.StackOpResponse, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("%w: 未选择要删除的镜像", v1.ErrBadRequest)
	}
	for _, ref := range refs {
		if !repository.ImageRefPattern.MatchString(ref) {
			return nil, fmt.Errorf("%w: 非法镜像引用 %s", v1.ErrBadRequest, ref)
		}
	}
	containers, err := s.repo.ResourceContainers(ctx)
	if err != nil {
		return nil, err
	}
	images, err := s.repo.ResourceImages(ctx, containers)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, ref := range refs {
		id := ""
		for _, image := range images {
			if imageRefMatchesID(ref, image.ID) {
				id = image.ID
			}
			for _, tag := range append(append([]string{}, image.RepoTags...), image.RepoDigests...) {
				if ref == tag || (!strings.Contains(ref, "@") && strings.LastIndex(ref, ":") <= strings.LastIndex(ref, "/") && ref+":latest" == tag) {
					id = image.ID
				}
			}
		}
		if id == "" {
			return nil, fmt.Errorf("镜像引用 %q 不存在，请刷新列表", ref)
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	output := ""
	for _, id := range ids {
		text, err := s.repo.ResourceAction(ctx, "images", id, "delete", "")
		output += text + "\n"
		if err != nil {
			return &v1.StackOpResponse{Output: strings.TrimSpace(output)}, err
		}
	}
	return &v1.StackOpResponse{Output: strings.TrimSpace(output)}, nil
}

func imageRefMatchesID(ref, id string) bool {
	raw := strings.TrimPrefix(id, "sha256:")
	return ref == id || ref == raw || len(ref) >= 12 && strings.HasPrefix(raw, ref)
}
