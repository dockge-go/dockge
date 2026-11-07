/* 初始演示数据（接真实后端时替换为 REST/SSE/WS 拉取）。
 * 类型对齐 docker run 参数面：端口协议/绑定 IP、env、挂载、网络、重启策略、资源限制。 */
export interface PortMap {
  PrivatePort?: number; PublicPort?: number;
  Type?: string; IP?: string; Proto?: 'tcp' | 'udp';
}
export interface MountMap { source: string; dest: string; mode: string }
export type RestartPolicy = 'no' | 'always' | 'unless-stopped' | 'on-failure';

export interface Container {
  Id: string; Names: string[]; Image: string; State: 'running' | 'exited' | 'paused';
  upHours?: number; exitCode?: number; exitHoursAgo?: number; exitMinAgo?: number;
  Created: number; Ports: PortMap[];
  system: boolean; _stats?: { cpuPct: number; cpuBar: number; memMB: number; memBar: number };
  /* docker run 参数面 */
  Env?: string[];
  Mounts?: MountMap[];
  Networks?: string[];
  RestartPolicy?: RestartPolicy;
  RestartCount?: number;
  Health?: 'starting' | 'healthy' | 'unhealthy';
  Cpus?: number;
  MemoryMB?: number;
  Entrypoint?: string;
  Cmd?: string;
}
export interface Image { Id: string; RepoTags: string[]; Repository: string; Size: number; Created: number; system: boolean }
export interface Volume { Name: string; Driver: string; Mountpoint: string; Usage: string; system: boolean; UsedBy?: { id: string; name: string }[] }
export interface Network { ID: string; Name: string; Driver: string; Subnet: string; Containers?: { name: string; ip: string }[]; system: boolean }
export interface Stack {
  id: string; name: string; compose_path: string; status: 'up' | 'down'; created: number;
  compose: string; composeDeployed: string | null; deployLog: string[];
}

const HOUR = 3600e3, DAY = 86400e3;

const tcp = (priv: number, pub?: number, ip = '0.0.0.0'): PortMap =>
  ({ PrivatePort: priv, PublicPort: pub, Type: 'tcp', IP: ip, Proto: 'tcp' });

export function defaultData() {
  return {
    containers: [
      {
        Id: 'a1b2c3d4e5f6', Names: ['/nginx-web'], Image: 'nginx:latest', State: 'running', upHours: 3,
        Created: Date.now() - 3 * HOUR, Ports: [tcp(80, 8080)],
        system: false, Env: ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin', 'NGINX_HOST=localhost'],
        Mounts: [{ source: './html', dest: '/usr/share/nginx/html', mode: 'ro' }],
        Networks: ['app_frontend'], RestartPolicy: 'unless-stopped', Cpus: 1, MemoryMB: 512
      },
      {
        Id: 'b2c3d4e5f6a1', Names: ['/postgres-main'], Image: 'postgres:16', State: 'running', upHours: 3,
        Created: Date.now() - 3 * HOUR, Ports: [tcp(5432, 5432)],
        system: false, Env: ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin', 'POSTGRES_PASSWORD=••••••', 'PGDATA=/var/lib/postgresql/data'],
        Mounts: [{ source: 'postgres_data', dest: '/var/lib/postgresql/data', mode: 'rw' }],
        Networks: ['app_backend'], RestartPolicy: 'always', Cpus: 2, MemoryMB: 2048, Health: 'healthy'
      },
      {
        Id: 'c3d4e5f6a1b2', Names: ['/redis-cache'], Image: 'redis:7-alpine', State: 'running', upHours: 3,
        Created: Date.now() - 3 * HOUR, Ports: [{ PrivatePort: 6379, Type: 'tcp', Proto: 'tcp' }],
        system: true, Env: ['PATH=/usr/local/bin'], Mounts: [{ source: 'redis_data', dest: '/data', mode: 'rw' }],
        Networks: ['app_backend'], RestartPolicy: 'no'
      },
      {
        Id: 'd4e5f6a1b2c3', Names: ['/worker-jobs'], Image: 'crate/worker:latest', State: 'exited', exitCode: 0, exitHoursAgo: 2,
        Created: Date.now() - 5 * HOUR, Ports: [],
        system: false, Env: ['PATH=/usr/local/bin', 'QUEUE=jobs'], Networks: ['app_backend'],
        RestartPolicy: 'on-failure', RestartCount: 7, Cmd: 'python worker.py'
      },
      {
        Id: 'e5f6a1b2c3d4', Names: ['/monitoring'], Image: 'prom/prometheus:latest', State: 'paused',
        Created: Date.now() - DAY, Ports: [tcp(9090, 9090)],
        system: true, Env: ['PATH=/usr/local/bin'], Networks: ['bridge'], RestartPolicy: 'no', Cpus: 0.5
      },
      {
        Id: 'f6a1b2c3d4e5', Names: ['/backup-svc'], Image: 'crate/backup:latest', State: 'exited', exitCode: 1, exitHoursAgo: 24,
        Created: Date.now() - 2 * DAY, Ports: [{ PrivatePort: 53, PublicPort: 5353, Type: 'udp', IP: '0.0.0.0', Proto: 'udp' }],
        system: false, Env: ['PATH=/usr/local/bin', 'SCHEDULE=0 3 * * *'], Networks: ['bridge'],
        RestartPolicy: 'on-failure', RestartCount: 3, Health: 'unhealthy', Cmd: '/bin/sh -c "backup.sh"'
      },
      {
        Id: 'a7b8c9d0e1f2', Names: ['/api-gateway'], Image: 'crate/api:3.2.1', State: 'running', upHours: 12,
        Created: Date.now() - 12 * HOUR, Ports: [tcp(3000, 3000)],
        system: false, Env: ['PATH=/usr/local/bin', 'DATABASE_URL=postgres://db:5432/app', 'LOG_LEVEL=info'],
        Mounts: [{ source: './config', dest: '/etc/app/config', mode: 'ro' }],
        Networks: ['app_frontend', 'app_backend'], RestartPolicy: 'unless-stopped', Cpus: 1.5, MemoryMB: 1024,
        Entrypoint: '/app/server'
      }
    ] as Container[],
    images: [
      { Id: 'sha256:abc123', RepoTags: ['nginx:latest'], Repository: 'nginx', Size: 187000000, Created: Date.now() - DAY, system: false },
      { Id: 'sha256:def456', RepoTags: ['postgres:16'], Repository: 'postgres', Size: 412000000, Created: Date.now() - 2 * DAY, system: false },
      { Id: 'sha256:ghi789', RepoTags: ['redis:7-alpine'], Repository: 'redis', Size: 32000000, Created: Date.now() - 3 * DAY, system: true },
      { Id: 'sha256:jkl012', RepoTags: ['crate/worker:latest'], Repository: 'crate/worker', Size: 245000000, Created: Date.now() - 4 * DAY, system: false },
      { Id: 'sha256:mno345', RepoTags: ['prom/prometheus:latest'], Repository: 'prom/prometheus', Size: 256000000, Created: Date.now() - 5 * DAY, system: true },
      { Id: 'sha256:pqr678', RepoTags: ['crate/api:3.2.1'], Repository: 'crate/api', Size: 198000000, Created: Date.now() - 6 * DAY, system: false },
      { Id: 'sha256:stu901', RepoTags: ['alpine:3.19'], Repository: 'alpine', Size: 7800000, Created: Date.now() - 7 * DAY, system: false },
      { Id: 'sha256:vwx234', RepoTags: ['node:20-slim'], Repository: 'node', Size: 267000000, Created: Date.now() - 8 * DAY, system: false }
    ] as Image[],
    volumes: [
      { Name: 'postgres_data', Driver: 'local', Mountpoint: '/var/lib/postgresql/data', Usage: '2.1GB', system: false },
      { Name: 'redis_data', Driver: 'local', Mountpoint: '/var/lib/redis', Usage: '128MB', system: true },
      { Name: 'nginx_conf', Driver: 'local', Mountpoint: '/etc/nginx/conf.d', Usage: '4KB', system: false },
      { Name: 'app_uploads', Driver: 'local', Mountpoint: '/data/uploads', Usage: '890MB', system: false }
    ] as Volume[],
    networks: [
      { ID: 'net001', Name: 'bridge', Driver: 'bridge', Subnet: '172.17.0.0/16', Containers: [{ name: '/monitoring', ip: '172.17.0.2' }, { name: '/backup-svc', ip: '172.17.0.3' }, { name: '/portainer-agent', ip: '172.17.0.4' }], system: true },
      { ID: 'net002', Name: 'host', Driver: 'host', Subnet: '—', Containers: [{ name: '/host-network', ip: '—' }], system: true },
      { ID: 'net003', Name: 'app_frontend', Driver: 'bridge', Subnet: '172.18.0.0/16', Containers: [{ name: '/nginx-web', ip: '172.18.0.2' }, { name: '/api-gateway', ip: '172.18.0.3' }], system: false },
      { ID: 'net004', Name: 'app_backend', Driver: 'bridge', Subnet: '172.19.0.0/16', Containers: [{ name: '/postgres-main', ip: '172.19.0.2' }, { name: '/redis-cache', ip: '172.19.0.3' }], system: false }
    ] as Network[],
    stacks: [
      {
        id: 'stack-1', name: 'web-app', compose_path: '/opt/stacks/web-app/compose.yaml', status: 'up', created: Date.now() - DAY,
        compose: 'services:\n  web:\n    image: nginx:latest\n    ports:\n      - "8080:80"\n    volumes:\n      - ./html:/usr/share/nginx/html\n  api:\n    image: crate/api:3.2.1\n    ports:\n      - "3000:3000"\n    environment:\n      - DATABASE_URL=postgres://db\n  db:\n    image: postgres:16\n    volumes:\n      - pg_data:/var/lib/postgresql/data\n\nvolumes:\n  pg_data:',
        composeDeployed: null, deployLog: [] as string[]
      },
      {
        id: 'stack-2', name: 'monitoring', compose_path: '/opt/stacks/monitoring/compose.yaml', status: 'down', created: Date.now() - 2 * DAY,
        compose: 'services:\n  prometheus:\n    image: prom/prometheus:latest\n    ports:\n      - "9090:9090"\n  grafana:\n    image: grafana/grafana:latest\n    ports:\n      - "3001:3000"',
        composeDeployed: null, deployLog: [] as string[]
      }
    ] as Stack[]
  };
}

export const SERVICE_LOGS: Record<string, string> = {
  web: '[web] INFO  nginx: starting\n[web] INFO  nginx: listening on :80\n[web] INFO  nginx: worker process started\n[web] INFO  GET / 200 (3ms)\n[web] INFO  GET /assets/app.js 200 (1ms)\n[web] WARN  upstream timeout on /api',
  api: '[api] INFO  Express server started on port 3000\n[api] INFO  Connected to postgres://db:5432/app\n[api] INFO  GET /api/health 200 (2ms)\n[api] INFO  POST /api/users 201 (45ms)\n[api] WARN  Slow query detected: 890ms\n[api] INFO  GET /api/users 200 (12ms)',
  db: '[db] INFO  PostgreSQL 16.2 started\n[db] INFO  database system is ready to accept connections\n[db] LOG   checkpoint starting\n[db] LOG   checkpoint complete\n[db] LOG   automatic vacuum of table "users"\n[db] INFO  connection received: host=api port=43210',
  prometheus: '[prom] INFO  Server is ready to receive web requests.\n[prom] INFO  Loading configuration file\n[prom] INFO  Completed loading of configuration file',
  grafana: '[grafana] INFO  Starting Grafana\n[grafana] INFO  HTTP Server Listen OK\n[grafana] WARN  [Alerting] Retrying backend connection'
};

export const defaultCompose = () => 'services:\n  app:\n    image: nginx:latest\n    ports:\n      - "8080:80"';
