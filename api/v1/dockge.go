// Package v1 定义 dockge 应用的 API 契约（DTO）。
package v1

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type MeUserData struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role,omitempty"` // admin / member
}

// 密码字段只做 required（防缺字段）；长度/强度规则统一由 service.ValidatePasswordStrength
// 判定——若在 DTO 放 min=6，绑定失败会落到笼统的「参数错误」，具体原因无法到达用户。
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

type SetupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponseData struct {
	AccessToken string     `json:"accessToken"`
	User        MeUserData `json:"user"`
}

// -------- 设置 --------

type SetGlobalEnvRequest struct {
	Content string `json:"content"`
}

// -------- 栈 --------

type StackSaveRequest struct {
	Name string `json:"name"`
	Yaml string `json:"yaml" binding:"required"`
	Env  string `json:"env"`
}

type StackSummaryData struct {
	Name            string `json:"name"`
	Status          int    `json:"status"`
	Managed         bool   `json:"managed"`
	ComposeFileName string `json:"composeFileName,omitempty"`
	ConfigFiles     string `json:"configFiles,omitempty"`
}

type StackListData struct {
	List []StackSummaryData `json:"list"`
}

type StackDetailData struct {
	Name            string           `json:"name"`
	Status          int              `json:"status"`
	Managed         bool             `json:"managed"`
	ComposeFileName string           `json:"composeFileName,omitempty"`
	ConfigFiles     string           `json:"configFiles,omitempty"`
	Yaml            string           `json:"yaml"`
	Env             string           `json:"env"`
	Containers      []StackContainer `json:"containers"`
	URLs            []string         `json:"urls,omitempty"`
}

type StackContainer struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Service string        `json:"service,omitempty"`
	Image   string        `json:"image,omitempty"`
	State   string        `json:"state"`
	Status  string        `json:"status"`
	Ports   []PortMapping `json:"ports,omitempty"`
}

// ContainerStat 是单个容器的即时资源占用（对齐 docker stats --format json）。
type ContainerStat struct {
	Name     string `json:"name"`
	CPUPerc  string `json:"cpuPerc"`
	MemUsage string `json:"memUsage"`
	MemPerc  string `json:"memPerc"`
	NetIO    string `json:"netIO"`
	BlockIO  string `json:"blockIO"`
}

type StackOpResponse struct {
	Output string `json:"output"`
}

// -------- docker --------

// PortMapping 是一条端口映射（hostIP:hostPort -> containerPort/proto）。
type PortMapping struct {
	HostIP        string `json:"hostIP,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// -------- images --------

// ImageData 是镜像列表的一行。
type ImageData struct {
	ID           string `json:"id"`
	Repository   string `json:"repository"`
	Tag          string `json:"tag"`
	Size         string `json:"size"`
	CreatedSince string `json:"createdSince"`
	InUse        bool   `json:"inUse"`
}

// ImageListData 是镜像列表响应。
type ImageListData struct {
	List []ImageData `json:"list"`
}

// ImagePullRequest 是手动拉取镜像请求。
type ImagePullRequest struct {
	Image string `json:"image" binding:"required"`
}

// ImageDeleteRequest 是批量删除镜像请求。
type ImageDeleteRequest struct {
	Images []string `json:"images" binding:"required"`
}

// -------- composerize --------

type ComposerizeRequest struct {
	DockerRunCommand string `json:"dockerRunCommand" binding:"required"`
}

type ComposerizeResponse struct {
	ComposeTemplate string `json:"composeTemplate"`
}
