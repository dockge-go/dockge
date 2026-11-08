import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=stripTypeScriptTypes(readFileSync(new URL('../src/lib/log-buffer.ts',import.meta.url),'utf8'));
const {LogBuffer}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
function feed(b,text){b.push(text);return b.snapshot().map(row=>row.raw);}
test('跨帧 ANSI 和未换行末行不丢失',()=>{
 const b=new LogBuffer(); assert.deepEqual(feed(b,'\x1b['),[]);
 assert.deepEqual(feed(b,'31m错误\x1b[0'),['错误']);
 assert.deepEqual(feed(b,'m\n末行'),['错误','末行']);
});
test('回车进度覆盖旧行而非粘连，OSC 不泄漏',()=>{
 const b=new LogBuffer(); feed(b,'Downloading 20%'); feed(b,'\r\x1b[');
 assert.deepEqual(feed(b,'2KDone\n\x1b]0;title\x1b\\日志'),['Done','日志']);
 b.clear(); assert.deepEqual(feed(b,'新会话'),['新会话']);
});
test('日志有界且保留真实 UTF-8 分片',()=>{
 const b=new LogBuffer(), d=new TextDecoder(), bytes=new TextEncoder().encode('中文\n');
 let rows=[]; for(const byte of bytes) rows=feed(b,d.decode(new Uint8Array([byte]),{stream:true}));
 assert.deepEqual(rows,['中文']); assert.equal(feed(b,'x\n'.repeat(600)).length,500);
});
test('部署长输出只保留最新显示行，超长末行同样有界',()=>{
 const b=new LogBuffer();
 for(let n=0;n<10000;n++) feed(b,`Downloading layer ${n}\n`);
 const rows=feed(b,'Done');
 assert.equal(rows.length,500); assert.equal(rows.at(-1),'Done');
 assert.ok(!rows.includes('Downloading layer 0'));
 assert.equal(feed(new LogBuffer(),'x'.repeat(70000))[0].length,65536);
});

test('很多长行的总字符预算有界，最新错误仍能展示',()=>{
 const b=new LogBuffer();
 for(let n=0;n<100;n++) feed(b,'x'.repeat(65536)+'\n');
 const rows=feed(b,'真实退出原因');
 assert.ok(rows.join('\n').length<=1024*1024); assert.equal(rows.at(-1),'真实退出原因');
 b.clear();assert.deepEqual(feed(b,'清空后的新日志'),['清空后的新日志']);
});
