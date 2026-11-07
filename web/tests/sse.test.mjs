import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const {readSSE,eventBytes} = await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(readFileSync(new URL('../src/lib/sse.ts',import.meta.url),'utf8'))).toString('base64'));
const response = text => new Response(new ReadableStream({start(c) {for(const b of new TextEncoder().encode(text)) c.enqueue(Uint8Array.of(b)); c.close();}}),{headers:{'Content-Type':'text/event-stream'}});
test('SSE 跨字节中文、CRLF、注释、多行 JSON 和帧默认事件',async()=>{
 const got=[];
 await readSSE(response(': heartbeat\r\nevent: log\r\ndata: {\r\ndata: "text":"真实中文"}\r\n\r\ndata: true\n\n'),e=>got.push(e));
 assert.deepEqual(got,[{event:'log',data:{text:'真实中文'}},{event:'message',data:true}]);
});
test('不完整帧、非法 JSON 和非 SSE 响应不可视为成功',async()=>{
 for(const text of ['event: done\ndata: true','data: invalid\n\n']) await assert.rejects(readSSE(response(text),()=>{}));
 await assert.rejects(readSSE(new Response('{}'),()=>{}),/不是 SSE/);
 await assert.rejects(readSSE(new Response('{"message":"资源不存在"}',{status:404}),()=>{}),/资源不存在/);
});
test('主动取消及事件处理异常释放流，不残留读取锁',async()=>{
 for (const already of [false,true]) {
  const controller=new AbortController(); let cancelled=false;
  const body=new ReadableStream({start(c){if(!already)c.enqueue(new TextEncoder().encode('data: true\n\n'));},cancel(){cancelled=true;}});
  if(already)controller.abort();
  await assert.rejects(readSSE(new Response(body,{headers:{'Content-Type':'text/event-stream'}}),()=>{throw new Error('handler stopped');},controller.signal));
  assert.equal(cancelled,true); assert.equal(body.locked,false);
 }
});
test('终端字节解码不修改中文及 ANSI',()=>{
 const original=Buffer.from('\x1b[31m中文\x1b[0m');
 assert.deepEqual(eventBytes(original.toString('base64')),new Uint8Array(original));
 assert.throws(()=>eventBytes(42));
});
