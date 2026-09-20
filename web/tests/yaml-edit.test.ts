import assert from "node:assert/strict";
import test from "node:test";

import { addService, listServices, readService, removeService, updateService } from "../src/lib/yaml-edit.ts";

const DOC = `# 顶部注释
services:
  web: # 行尾注释
    image: nginx:latest
    ports:
      - "8080:80"
  db:
    image: postgres:16
networks:
  edge:
`;

test("listServices returns service names", () => {
  assert.deepEqual(listServices(DOC), ["web", "db"]);
  assert.deepEqual(listServices("not: yaml"), []);
});

test("addService appends skeleton with restart policy and keeps comments", () => {
  const out = addService(DOC, "cache");
  assert.match(out, /cache:/);
  assert.match(out, /restart: unless-stopped/);
  assert.match(out, /# 顶部注释/);
  assert.match(out, /# 行尾注释/);
  assert.equal(listServices(out).length, 3);
});

test("addService is idempotent on existing name", () => {
  assert.equal(addService(DOC, "web"), DOC);
});

test("removeService drops the service and keeps others", () => {
  const out = removeService(DOC, "web");
  assert.deepEqual(listServices(out), ["db"]);
  assert.match(out, /# 顶部注释/);
});

test("updateService rewrites scalar and list fields, keeps comments", () => {
  const out = updateService(DOC, "web", {
    image: "nginx:1.27",
    ports: ["9090:80"],
    volumes: ["data:/var/www"],
    restart: "always",
    environment: ["KEY=value"],
    dependsOn: ["db"],
    networks: ["edge"],
  });
  assert.match(out, /image: nginx:1\.27/);
  assert.match(out, /- 9090:80/);
  assert.match(out, /- data:\/var\/www/);
  assert.match(out, /restart: always/);
  assert.match(out, /- KEY=value/);
  // 回归：compose 键是 depends_on，写错键名的服务依赖会被 compose 静默忽略
  assert.match(out, /depends_on:\n\s+- db/);
  assert.doesNotMatch(out, /dependsOn/);
  assert.match(out, /networks:\n\s+- edge/);
  assert.match(out, /# 行尾注释/);
  assert.match(out, /# 顶部注释/);
});

test("updateService on missing service is a no-op", () => {
  assert.equal(updateService(DOC, "ghost", { image: "x" }), DOC);
});

test("readService returns current fields", () => {
  assert.deepEqual(readService(DOC, "web"), {
    image: "nginx:latest",
    ports: ["8080:80"],
    volumes: undefined,
    restart: undefined,
    environment: undefined,
    dependsOn: undefined,
    networks: undefined,
    containerName: undefined,
  });
  assert.deepEqual(readService(DOC, "ghost"), {});
});

test("readService maps depends_on to dependsOn", () => {
  const doc = `services:\n  web:\n    image: nginx\n    depends_on:\n      - db\n`;
  assert.deepEqual(readService(doc, "web").dependsOn, ["db"]);
});

test("顶层网络条目：内部/外部往返（上游 NetworkInput 等价物）", async () => {
  const { listTopLevelNetworkEntries, listTopLevelNetworks, setTopLevelNetworkEntries } =
    await import("../src/lib/yaml-edit.ts");
  assert.deepEqual(listTopLevelNetworks(DOC), ["edge"]);

  const entries = listTopLevelNetworkEntries(DOC);
  assert.deepEqual(entries, [{ name: "edge", external: false }]);

  const out = setTopLevelNetworkEntries(DOC, [
    { name: "edge", external: false },
    { name: "lan", external: false },
    { name: "shared", external: true },
  ]);
  assert.deepEqual(listTopLevelNetworks(out), ["edge", "lan", "shared"]);
  assert.deepEqual(listTopLevelNetworkEntries(out), [
    { name: "edge", external: false },
    { name: "lan", external: false },
    { name: "shared", external: true },
  ]);
  assert.match(out, /external: true/);
  assert.match(out, /# 顶部注释/);

  // 空名跳过；空列表删除 networks 段
  const cleared = setTopLevelNetworkEntries(out, [{ name: "  ", external: false }]);
  assert.deepEqual(listTopLevelNetworks(cleared), []);
  assert.doesNotMatch(cleared, /networks:/);
});

test("格式化：规整缩进与流式写法，语义不变、注释保留", async () => {
  const { formatYaml } = await import("../src/lib/yaml-edit.ts");

  // 用户手写形态：子键深漂移（10/8 空格）+ 流式数组跨行——语法合法但难读
  const ugly = `services:
  mongodb:
    image: mongo:8
    environment:
          MONGO_INITDB_ROOT_USERNAME: admin
    healthcheck:
        test:
          [
            "CMD-SHELL",
            "mongosh --eval 'db.adminCommand(\\\"ping\\\").ok'"
          ]
        interval: 10s
`;
  const out = formatYaml(ugly);
  assert.ok(out && out.length > 0);
  // 语法合法且语义保留
  const { parseDocument } = await import("yaml");
  const doc = parseDocument(out);
  assert.equal(doc.errors.length, 0);
  assert.equal(doc.get("in", false) ?? doc.toJS().services.mongodb.image, "mongo:8");
  assert.equal(doc.toJS().services.mongodb.healthcheck.interval, "10s");
  assert.deepEqual(doc.toJS().services.mongodb.healthcheck.test, [
    "CMD-SHELL",
    "mongosh --eval 'db.adminCommand(\"ping\").ok'",
  ]);
  // 缩进统一为 2 空格（顶层键无前导空白、service 键 2 空格、
  // environment 的 10 空格漂移子键规整为 4 空格——toString 对合法漂移重排列）
  assert.match(out, /^services:\n  mongodb:\n    image: mongo:8/);
  assert.match(out, /\n      MONGO_INITDB_ROOT_USERNAME: admin/);
  assert.doesNotMatch(out, /\n {7,}MONGO_INITDB_ROOT_USERNAME/);

  // 注释保留
  const commented = formatYaml("# 顶部注释\nservices: # 行尾\n  a:\n    image: nginx\n");
  assert.match(commented, /# 顶部注释/);
  assert.match(commented, /# 行尾/);

  // 语法错误 → null（格式化不修复语法）
  assert.equal(formatYaml("services:\n  a: [unclosed"), null);
});

test("composeDefects：单次解析收集语法/网络/卷/依赖缺陷", async () => {
  const { composeDefects } = await import("../src/lib/yaml-edit.ts");

  // 用户踩坑形态：未定义网络 + 未定义具名卷 + 残留依赖，一次全查出
  const bad = `services:
  mongodb:
    image: mongo:8
    volumes:
      - mongodb_data:/data/db
    networks:
      - backend
    depends_on:
      - db
`;
  const d = composeDefects(bad);
  assert.deepEqual(d.networks, ["backend"]);
  assert.deepEqual(d.volumes, ["mongodb_data"]);
  assert.deepEqual(d.dependsOn, ["db"]);

  // bind 挂载/匿名卷/已定义引用/字典与长语法写法不误报；语法错误单列
  const ok = `services:
  a:
    image: nginx
    volumes:
      - /etc/localtime:/etc/localtime:ro
      - ./site:/usr/share/nginx/html
      - db_data:/data
    networks:
      edge: {}
  b:
    image: redis
    depends_on:
      - service: a
networks:
  edge:
    external: true
volumes:
  db_data: {}
`;
  const d2 = composeDefects(ok);
  assert.deepEqual([d2.networks, d2.volumes, d2.dependsOn], [[], [], []]);
  assert.equal(composeDefects("services: [unclosed").syntax.length > 0, true);
  assert.deepEqual(composeDefects("services:\n  a:\n    image: nginx\n"), { syntax: "", syntaxLine: 0, networks: [], volumes: [], dependsOn: [] });
  // 语法错误带行号（供编辑器行内标注）
  const syn = composeDefects("services:\n  a:\n    image: [unclosed");
  assert.ok(syn.syntax.length > 0);
  assert.ok(syn.syntaxLine >= 2);
});

test("闭环：选网络自动补顶层定义（本机网络标记 external）", async () => {
  const { ensureTopLevelNetworks, listTopLevelNetworkEntries, composeDefects } =
    await import("../src/lib/yaml-edit.ts");

  const doc = `services:\n  web:\n    image: nginx\n    networks:\n      - edge\nnetworks:\n  edge:\n`;
  const host = new Set(["traefik_proxy", "bridge"]);

  // 选了本机网络 traefik_proxy + 新名字 lan → 补定义：traefik_proxy external，lan 普通
  const out = ensureTopLevelNetworks(doc, ["edge", "traefik_proxy", "lan"], host);
  const entries = listTopLevelNetworkEntries(out);
  assert.deepEqual(
    entries.map((e) => `${e.name}${e.external ? ":external" : ""}`),
    ["edge", "traefik_proxy:external", "lan"],
  );
  // 闭环出口：补完定义后未定义网络校验必然通过
  assert.deepEqual(composeDefects(out).networks, []);

  // 全部已定义 → 原样返回（引用相等，不产生多余重排）
  assert.equal(ensureTopLevelNetworks(out, ["edge", "lan"], host), out);

  // 手写的既有定义不被改写（lan 已是非 external，即使在本机集合里也不动）
  const out2 = ensureTopLevelNetworks(out, ["lan"], host);
  assert.equal(out2, out);
});

test("闭环：表单输入具名卷自动补顶层 volumes 声明", async () => {
  const { ensureTopLevelVolumes } = await import("../src/lib/yaml-edit.ts");

  const doc = `services:
  mongodb:
    image: mongo:8
    volumes:
      - mongodb_data:/data/db
`;
  const out = ensureTopLevelVolumes(doc, ["mongodb_data:/data/db"]);
  assert.match(out, /volumes:\n  mongodb_data: \{\}/);

  // bind 挂载与匿名卷不补声明；已定义不动（原样返回）
  const mixed = ensureTopLevelVolumes(doc, ["./site:/usr/share/nginx/html", "/data/cache"]);
  assert.equal(mixed, doc);
  assert.equal(ensureTopLevelVolumes(out, ["mongodb_data:/data/db"]), out);

  // 顶层尚无 volumes 段也能创建
  const noSection = "services:\n  a:\n    image: nginx\n";
  const created = ensureTopLevelVolumes(noSection, ["cfg:/etc/app"]);
  assert.match(created, /volumes:\n  cfg: {}/);
});

test("网络卡与表单联动：顶层改名/删除同步服务引用", async () => {
  const { renameTopLevelNetwork, removeTopLevelNetwork, composeDefects, listTopLevelNetworks } =
    await import("../src/lib/yaml-edit.ts");

  const doc = `services:
  web:
    image: nginx
    networks:
      - lan
      - edge
  db:
    image: redis
    networks:
      lan: {}
networks:
  lan:
  edge:
    external: true
`;

  // 改名 lan → backend：短语法与字典写法的引用都替换，外部定义保留
  const renamed = renameTopLevelNetwork(doc, "lan", "backend");
  const js = (await import("yaml")).parseDocument(renamed).toJS();
  assert.deepEqual(js.services.web.networks, ["backend", "edge"]);
  assert.ok(js.services.db.networks.backend);
  assert.deepEqual(listTopLevelNetworks(renamed), ["backend", "edge"]);
  assert.deepEqual(composeDefects(renamed).networks, []);

  // 删除 backend：全部引用移除，db.networks 清空则删键
  const removed = removeTopLevelNetwork(renamed, "backend");
  const js2 = (await import("yaml")).parseDocument(removed).toJS();
  assert.deepEqual(js2.services.web.networks, ["edge"]);
  assert.equal(js2.services.db.networks, undefined);
  assert.deepEqual(composeDefects(removed).networks, []);

  // 空名/不存在：原样返回
  assert.equal(renameTopLevelNetwork(doc, "", "x"), doc);
  assert.equal(removeTopLevelNetwork(doc, "nope"), doc);
});

test("字典写法保护：environment/depends_on/networks 的 dict 形态表单退位（防覆写丢数据）", async () => {
  const { hasLongSyntax, readService } = await import("../src/lib/yaml-edit.ts");

  // 用户实际形态：environment 为字典（mongo 初始化变量）
  const doc = `services:
  mongodb:
    image: mongo:8
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: pwd
`;
  assert.equal(hasLongSyntax(doc, "mongodb", "environment"), "long");
  assert.equal(readService(doc, "mongodb").environment, undefined);

  // depends_on 字典（condition 写法）与 networks 字典（aliases 写法）同样退位
  const doc2 = `services:
  a:
    image: nginx
    depends_on:
      db:
        condition: service_healthy
    networks:
      front:
        aliases: [web]
`;
  assert.equal(hasLongSyntax(doc2, "a", "dependsOn"), "long");
  assert.equal(hasLongSyntax(doc2, "a", "networks"), "long");

  // 列表短语法不受影响（表单正常编辑）
  assert.equal(hasLongSyntax("services:\n  a:\n    image: nginx\n    environment:\n      - K=V\n", "a", "environment"), undefined);
});
