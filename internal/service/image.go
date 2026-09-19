// 镜像用例：列表、手动拉取与批量删除；引用合法性在此层校验（CLI 参数注入防线）。
package service

import (
	"context"
	"fmt"
	"io"

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
	return &imageService{Service: do.MustInvoke[*Service](i)}, nil
}

type imageService struct {
	*Service
}

// List 返回本机镜像列表（含使用中标记）。
func (s *imageService) List(ctx context.Context) ([]v1.ImageData, error) {
	images, err := s.repo.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]v1.ImageData, 0, len(images))
	for _, img := range images {
		list = append(list, v1.ImageData{
			ID: img.ID, Repository: img.Repository, Tag: img.Tag,
			Size: img.Size, CreatedSince: img.CreatedSince, InUse: img.InUse,
		})
	}
	return list, nil
}

// PullStream 流式拉取镜像，compose 输出逐行写入 w。
func (s *imageService) PullStream(ctx context.Context, w io.Writer, ref string) error {
	if !repository.ImageRefPattern.MatchString(ref) {
		return fmt.Errorf("%w: 非法镜像引用", v1.ErrBadRequest)
	}
	return s.repo.PullImageStream(ctx, w, ref)
}

// Delete 批量删除镜像，失败时带回 docker 输出。
func (s *imageService) Delete(ctx context.Context, refs []string) (*v1.StackOpResponse, error) {
	if len(refs) == 0 {
		return nil, v1.ErrBadRequest
	}
	for _, ref := range refs {
		if !repository.ImageRefPattern.MatchString(ref) {
			return nil, fmt.Errorf("%w: 非法镜像引用 %s", v1.ErrBadRequest, ref)
		}
	}
	output, err := s.repo.DeleteImages(ctx, refs)
	if err != nil {
		return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)},
			fmt.Errorf("%w: %s", v1.ErrDockerError, err.Error())
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}
