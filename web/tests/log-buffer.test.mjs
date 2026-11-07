import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=stripTypeScriptTypes(readFileSync(new URL('../src/lib/log-buffer.ts',import.meta.url),'utf8'));
const {LogBuffer}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
test('跨帧 ANSI 和未换行末行不丢失',()=>{
 const b=new LogBuffer(); assert.deepEqual(b.push('\x1b['),[]);
 assert.deepEqual(b.push('31m错误\x1b[0'),['错误']);
 assert.deepEqual(b.push('m\n末行'),['错误','末行']);
});
test('回车进度覆盖旧行而非粘连，OSC 不泄漏',()=>{
 const b=new LogBuffer(); b.push('Downloading 20%'); b.push('\r\x1b[');
 assert.deepEqual(b.push('2KDone\n\x1b]0;title\x1b\\日志'),['Done','日志']);
 b.clear(); assert.deepEqual(b.push('新会话'),['新会话']);
});
test('日志有界且保留真实 UTF-8 分片',()=>{
 const b=new LogBuffer(), d=new TextDecoder(), bytes=new TextEncoder().encode('中文\n');
 let rows=[]; for(const byte of bytes) rows=b.push(d.decode(new Uint8Array([byte]),{stream:true}));
 assert.deepEqual(rows,['中文']); assert.equal(b.push('x\n'.repeat(600)).length,500);
});
