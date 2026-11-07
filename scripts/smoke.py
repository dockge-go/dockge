"""运行中实例的真实运行时冒烟；清理范围限定为本次创建的栈。"""
import json
import os
import re
import signal
import sys
import uuid
from urllib.error import HTTPError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


class Smoke:
    def __init__(self):
        self.base = os.environ.get('BASE', 'http://127.0.0.1:5001').rstrip('/')
        if urlparse(self.base).scheme not in ('http', 'https'):
            raise ValueError('BASE 必须是 http:// 或 https:// 实例地址')
        self.token = os.environ.get('DOCKGE_TOKEN', '')
        self.stack = 'smoke-' + uuid.uuid4().hex[:16]
        self.created = False
        self.passed = 0

    def request(self, method, path, data=None, expected=200, auth=True, stream=False):
        headers = {}
        if auth and self.token:
            headers['Authorization'] = 'Bearer ' + self.token
        body = None
        if data is not None:
            headers['Content-Type'] = 'application/json'
            body = json.dumps(data).encode()
        req = Request(self.base + '/v1' + path, body, headers, method=method)
        try:
            response = urlopen(req, timeout=660 if stream else 300)
        except HTTPError as error:
            response = error
        with response:
            status = response.code
            text = response.read().decode('utf-8', errors='replace')
        if status != expected:
            # 不回显请求体、令牌或密码。
            raise RuntimeError(f'{method} {path}: HTTP {status}，预期 {expected}；{text}')
        if stream:
            event, output, complete = '', '', False
            for frame in text.replace('\r\n', '\n').split('\n\n'):
                data = []
                for line in frame.split('\n'):
                    if line.startswith('event:'): event = line[6:].strip()
                    elif line.startswith('data:'): data.append(line[5:].lstrip())
                if not data: continue
                value = json.loads('\n'.join(data))
                if event == 'output': output += value
                elif event == 'error': raise RuntimeError(f'{method} {path}: {output}\n{value}')
                elif event == 'done' and value is True: complete = True
            if not complete: raise RuntimeError(f'{method} {path}: SSE 未收到完成确认')
            return output
        if not text or expected != 200:
            return None
        try:
            payload = json.loads(text)
        except json.JSONDecodeError as error:
            raise RuntimeError(f'{method} {path}: JSON 响应无法解析：{error}') from error
        return payload.get('data')

    def check(self, name, condition=True):
        if not condition:
            raise RuntimeError('断言失败：' + name)
        self.passed += 1
        print('✓ ' + name, flush=True)

    def run(self):
        self.request('GET', '/health', auth=False)
        self.check('健康检查')
        self.request('GET', '/stacks', expected=401, auth=False)
        self.check('栈接口鉴权')
        if not self.token:
            username = os.environ.get('DOCKGE_USER', '')
            password = os.environ.get('DOCKGE_PASS', '')
            if not username or not password:
                raise ValueError('请提供 DOCKGE_TOKEN，或同时提供 DOCKGE_USER 和 DOCKGE_PASS；没有默认密码')
            result = self.request('POST', '/login', {'username': username, 'password': password}, auth=False)
            self.token = result['accessToken']
        self.request('GET', '/me')
        self.check('当前账号')
        image = os.environ.get('SMOKE_IMAGE', 'nginx:alpine')
        yaml = ('services:\n  web:\n    image: ' + json.dumps(image) + '\n'
                '    volumes:\n      - data:/usr/share/nginx/html\nvolumes:\n  data:\n')
        self.request('POST', '/stacks', {'name': self.stack, 'yaml': yaml, 'env': ''})
        self.created = True
        self.check('创建专属栈（不占用宿主端口）')
        path = '/stacks/' + self.stack
        data = self.request('GET', path)
        self.check('读取 YAML', data['yaml'] == yaml)
        self.request('PUT', path, {'name': self.stack, 'yaml': yaml + '# smoke\n', 'env': ''})
        self.check('保存 YAML')
        self.request('PUT', path, {'name': self.stack, 'yaml': 'services: ['}, expected=400)
        self.check('拒绝非法 YAML')
        for operation, status in [('start', 3), ('restart', 3), ('stop', 4), ('start', 3)]:
            self.request('POST', path + '/' + operation, stream=True)
            state = self.request('GET', path)['status']
            self.check(operation + ' 状态', state == status)
        stats = self.request('GET', path + '/stats')
        self.check('容器统计', isinstance(stats, list) and len(stats) > 0)
        for setting in ('globalenv', 'primaryhostname'):
            self.request('GET', '/settings/' + setting)
            self.check('读取设置 ' + setting)
        stacks = self.request('GET', '/stacks')['list']
        self.check('列表包含本次专属栈', any(row['name'] == self.stack for row in stacks))
        self.request('GET', '/system')
        self.check('服务器指标接口')
        for endpoint in ('/users', '/agents', '/console/enabled', '/version/check'):
            self.request('GET', endpoint, expected=404)
            self.check('已移除接口 ' + endpoint)
        plan = self.request('GET', path + '/delete-plan')
        self.check('删除计划无阻塞', not plan['blockers'])
        self.check('删除计划含容器、命名卷和网络', all(plan[key] for key in ('containers', 'volumes', 'networks')))
        self.cleanup()
        self.request('GET', path, expected=404)
        self.check('删除后栈不存在')
        print(f'冒烟通过：{self.passed} 项')

    def cleanup(self):
        if self.created:
            self.request('DELETE', '/stacks/' + self.stack)
            self.created = False
            print('✓ 已清理本次专属栈 ' + self.stack, flush=True)


def main():
    smoke = Smoke()
    def terminate(_signum, _frame):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, terminate)
    failed = False
    try:
        smoke.run()
    except (Exception, KeyboardInterrupt) as error:
        failed = True
        print('冒烟失败：' + (str(error) or '已中断'), file=sys.stderr)
    finally:
        try:
            smoke.cleanup()
        except Exception as error:
            failed = True
            print(f'清理失败：{error}；请在面板核对专属栈 {smoke.stack}，未清理共享资源', file=sys.stderr)
    return int(failed)


if __name__ == '__main__':
    sys.exit(main())
