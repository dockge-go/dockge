import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const moduleURL = text => 'data:text/javascript;base64,' + Buffer.from(stripTypeScriptTypes(text)).toString('base64');
const sseURL = moduleURL(readFileSync(new URL('../src/lib/sse.ts',import.meta.url),'utf8'));
const bufferURL = moduleURL(readFileSync(new URL('../src/lib/log-buffer.ts',import.meta.url),'utf8'));
const source = readFileSync(new URL('../src/lib/api.ts',import.meta.url),'utf8')
 .replace("import { AuthAPI } from './auth';", "const AuthAPI = { token: () => 'isolated-test-token' };")
 .replace("from './sse'", `from '${sseURL}'`)
 .replace("from './log-buffer'", `from '${bufferURL}'`);
const {stream} = await import(moduleURL(source));
function responseChunks(t,text,status=200) {
 const original = globalThis.fetch;
 t.after(() => {globalThis.fetch = original});
 const bytes = new TextEncoder().encode(text);
 globalThis.fetch = async () => new Response(new ReadableStream({start(controller) {
  for (const byte of bytes) controller.enqueue(new Uint8Array([byte]));
  controller.close();
 }}),{status,headers:{'Content-Type':status===200?'text/event-stream':'application/json'}});
}
test('流式失败保留真实 stderr 与退出码，提示不泄漏 ANSI',async t => {
 responseChunks(t,`event: output\ndata: ${JSON.stringify('\x1b[31mservices.app Additional property unsupported_field is not allowed\x1b[0m\n')}\n\nevent: error\ndata: ${JSON.stringify('compose start 失败（退出码 1）')}\n\n`);
 let raw = '';
 await assert.rejects(stream('/stacks/test/start',text => {raw += text}),error => {
  assert.match(error.message,/unsupported_field is not allowed/);
  assert.match(error.message,/退出码 1/);
  assert.ok(!error.message.includes('\x1b'));
  return true;
 });
 assert.ok(raw.includes('\x1b[31m'));
});
test('成功流跨字节中文输出且末行完整，不产生伪失败',async t => {
 responseChunks(t,`event: output\ndata: ${JSON.stringify('真实中文输出\nStarted')}\n\nevent: done\ndata: true\n\n`);
 let output = '';
 await stream('/stacks/test/start',text => {output += text});
 assert.equal(output,'真实中文输出\nStarted');
});
test('HTTP 错误保留服务端具体明细',async t => {
 responseChunks(t,JSON.stringify({message:'栈 missing 不存在'}),404);
 await assert.rejects(stream('/stacks/missing/start',() => {}),/栈 missing 不存在/);
});

test('断流没有 done 不能误判成功',async t => {
 responseChunks(t,'event: output\ndata: "Started"\n\n');
 await assert.rejects(stream('/stacks/test/start',()=>{}),/未收到完成确认/);
});
test('运行时输出包含 [error] 不再被误判为协议错误',async t => {
 responseChunks(t,'event: output\ndata: "[error] application log\\n"\n\nevent: done\ndata: true\n\n');
 await stream('/stacks/test/start',()=>{});
});
