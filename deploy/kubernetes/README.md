# Kubernetes 部署与 OIDC 注入

这是**单机 Docker Compose 管理面板在 Kubernetes 中的部署示例**，不是原生 Kubernetes 工作负载管理器。面板编辑的是 `compose.yaml`，容器命令行与日志来自所挂载的本机运行时。没有新增用户、Prometheus、ServiceMonitor、集群 Agent 或 Kubernetes 管理权限。

## 部署前提（不能省略）

1. 选定**一个**节点，宿主上必须已有可访问的 Docker Engine socket。此示例镜像使用 `deploy/Dockerfile` 中的 Docker CLI 与 Compose 插件。只有 Kubernetes/containerd 并不等于已安装 Docker Engine；没有 Docker socket 时该示例不能运行核心功能。使用其他运行时时，需自行提供对应 CLI、Compose 实现及连接挂载，服务端通过 `repository.Runtime` 调用。
2. Docker socket 给予面板接近宿主 root 的权限；仅限可信管理员使用，必须保护网络入口和 Secret。这里不设置 `privileged: true`，不向 Pod 挂载 Kubernetes ServiceAccount 令牌。
3. 栈、数据库、服务端口在容器内部固定为 `/opt/stacks`、`/app/data`、`5001`。只改 `hostPath.path` 等宿主/存储侧映射，**不要用环境变量改变内部路径**。Compose 的 bind mount 路径由宿主运行时解析：换宿主栈路径后，必须确认 YAML 中宿主绑定源真实存在；优先使用命名卷。
4. `replicas: 1`、`Recreate` 和唯一节点标签保证单个 bbolt 写入实例，不支持跨节点调度、多个副本或水平扩容。数据备份必须包含选定节点的栈目录及其隐藏 `.dockge-data`；容器内 `/app/data` 自动连接该隐藏目录。旧部署独立挂载的 `/var/lib/dockge` 可继续使用，迁移前必须停机备份。
5. 配置有效的 HTTPS Ingress、证书和 OIDC 客户端。将 `configmap.yaml`、`ingress.example.yaml` 的示例域名替换为实际地址。在提供方登记精确回调地址：`https://实际面板域名/v1/auth/oidc/callback`。

## OIDC 安全模型

- 配置经 ConfigMap/Secret 环境注入；启用时配置缺失将使启动失败，不静默回退到密码登录。
- `DOCKGE_OIDC_SUBJECT` 必须是允许的唯一身份在该 issuer 下的稳定 `sub`。不能填邮箱或显示用户名；提供方中的其他用户，即使成功认证，也会被面板拒绝。
- 使用授权码 + PKCE/S256，验证 state、nonce、ID Token 签名、issuer、audience、azp、过期时间与 subject。只请求 `openid`，不保存外部 access/refresh token。
- 新数据库在该身份首次成功登录时初始化一个管理账号；已有数据库映射到原始管理员。不会为每个 OIDC 用户创建账号。
- 启用 OIDC 后，本地初始化、密码登录、改密和免认证模式均禁用。切换模式或 issuer/client/subject 后旧模式会话不能用于新模式。
- 回调 JWT 不放进 URL；经 60 秒、一次性 HttpOnly 票据和同源 POST 交给前端。本地会话最多一小时，且不超过本次 ID Token 有效期。过期需重新走 OIDC 登录。
- Cookie 安全属性以配置回调地址为准；生产必须 HTTPS。仅回环地址允许 HTTP 用于本机开发，不提供跳过 TLS/签名验证的开关。
- Pod 更新清空未完成登录事务，用户重新点击登录即可。Secret/ConfigMap 更新后需要 `kubectl rollout restart deployment/dockge -n dockge`。

## 操作步骤

清单固定使用 Docker Hub 镜像 `makeshit/dockge:0.2.6`，拉取策略为 `IfNotPresent`，无需在节点手动导入源码镜像。两挂载布局从 `0.2.6` 起支持，旧 `0.2.5` 缺少新入口，不能用于此清单。部署前确认对应版本的 [Release 工作流](https://github.com/dockge-go/dockge/actions/workflows/release.yml)已成功完成镜像发布。

GHCR 同版本镜像为 `ghcr.io/dockge-go/dockge:0.2.6`；核对访问权限后可替换清单的 `image`。生产升级时固定已发布版本或摘要，不使用浮动 `latest`。节点必须能访问所选镜像仓库；Docker Engine 与 K8s/containerd 的镜像存储彼此独立。

GHCR 包首次创建通常为私有，公开仓库并不自动保证镜像公开。发布管理员需在组织的 Packages → dockge → Package settings 中将可见性设为 Public，再检查匿名拉取。若选择保持私有，必须通过现有凭据管理系统在 `dockge` 命名空间创建 `kubernetes.io/dockerconfigjson` 类型的 `ghcr` Secret，并在 Pod 的 `spec.imagePullSecrets` 引用它；凭据需要 `read:packages`，不要写进清单或版本控制。默认 Docker Hub 公共镜像作为无需仓库凭据的部署来源。

```bash
# 只给一个具备运行时的节点打标签。
kubectl label node YOUR_RUNTIME_NODE dockge-runtime=true
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl apply -f deploy/kubernetes/configmap.yaml
```

编辑 `secret.example.yaml`，填入实际 client secret、唯一 subject 与独立生成的面板 JWT 签名密钥（至少 32 字节随机值，不能保留默认值或复用 client secret）；在受保护的本地副本中保存，部署后不要提交真实值。也可通过现有 Secret 管理系统生成同名 Secret：

```bash
kubectl apply -f /安全位置/dockge-oidc-secret.yaml
kubectl apply -f deploy/kubernetes/deployment.yaml -f deploy/kubernetes/service.yaml
# 先准备 dockge-tls Secret，再应用修改后的 Ingress。
kubectl apply -f /安全位置/dockge-ingress.yaml
kubectl rollout status deployment/dockge -n dockge
```

不要直接 `kubectl apply -f deploy/kubernetes/`：`*.example.yaml` 是需替换的示例，必须先填写实际配置。

如果出现 `ImagePullBackOff`，先执行 `kubectl describe pod -n dockge` 检查具体原因：镜像版本是否存在、GHCR 可见性/拉取凭据、节点网络与架构。若 Pod 启动失败，使用 `kubectl logs -n dockge deployment/dockge --previous` 查看 OIDC 配置或运行时挂载错误；不要禁用认证来掩盖配置错误。

浏览器访问真实 HTTPS 域名，点击 OIDC 登录。进入面板后先查看部署自检；再验证新增 Compose YAML、保存、部署、容器终端、实时日志、停止和删除。

## 删除与数据归属

- 编排栈仅来自 stacks 内的真实配置目录，Dockge 自身和目录外项目不展示、不通过栈 API 操作；`.dockge-data` 不算栈。
- 删除栈会先显示计划；删除专属容器（含孤儿）、卷、网络、镜像与整个栈目录（包括 `compose.yaml`、`.env` 和目录内文件）。共享、external、栈外 bind mount 数据会明确保留；不能以“删栈”为由破坏其他业务。
- 独立容器、镜像、卷、网络可以在对应页面删除；仍被引用或内建网络等无法删除时返回真实原因。先停止/删除引用对象，再删除资源，不提供全主机强制清空。
- 面板不会从页面删除正在运行的自身容器，也不会强行删除唯一账号造成公开初始化入口。账号/设置重置通过停机后删除数据库完成。
- 卸载面板：`kubectl delete namespace dockge` 会删除 Deployment、Service、Ingress、ConfigMap 与 Secret；**hostPath 数据和面板管理的 Compose 容器仍在宿主，不会因删 Pod/命名空间而消失**。
- 完整退场：先在面板逐个删除待清理栈并确认保留清单，停机/卸载后，管理员核实并手工删除选定节点的栈数据与面板数据目录。Kubernetes 不会替你删除宿主目录；不提供误删整台主机的脚本。

## 验证与服务器指标

仓库自动化测试覆盖 OIDC 配置与协议，以及固定镜像版本、路径、副本数、权限、认证注入与健康探针；Actions 使用 kubeconform 对所有清单（含待填写的配置示例）执行严格 Kubernetes 1.31 schema 校验。静态校验不等于真实集群验收。生产部署仍需使用实际身份提供方和集群验证登录回调、唯一身份限制、Compose CRUD、终端及日志。

默认仅挂载运行时 socket、stacks，不增加宿主 procfs 文件、hostPID、privileged 或指标 Agent。标准 Linux procfs 提供选定节点内核全局计数，本机运行时提供宿主身份；LXCFS 等虚拟化来源不能冒充节点指标。磁盘空间为 `/opt/stacks` 所在文件系统，不代表整个集群。采样周期与未知值处理参见 [服务器指标说明](../../README.zh-CN.md#服务器指标)。
