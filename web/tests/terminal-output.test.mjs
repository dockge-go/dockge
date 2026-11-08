import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=stripTypeScriptTypes(readFileSync(new URL('../src/lib/terminal-output.ts',import.meta.url),'utf8'),{mode:'transform'});
const {TerminalBuffer,TerminalRenderer,writeTerminal}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
test('部署显示尾部有界，ANSI 和跨帧进度控制码原样保留',()=>{
 const b=new TerminalBuffer();b.push('\x1b[3');
 assert.equal(b.push('1m中文\x1b[0m\rDownloading\x1b[2K\n').join('\n'),'\x1b[31m中文\x1b[0m\rDownloading\x1b[2K\n');
 for(let n=0;n<1000;n++)b.push(`layer ${n}\n`);
 const rows=b.push('Done');assert.equal(rows.length,500);assert.equal(rows.at(-1),'Done');assert.equal(rows.includes('layer 0'),false);
 assert.deepEqual(new TerminalBuffer().push('\n'),['','']);
});
test('超长输出截断不泄漏半个 ANSI、OSC 或 Unicode 字符',()=>{
 const limit=1024*1024;
 for(const prefix of ['\x1b[31m','\x1b]0;title\x07','\x1b]0;title\x1b\\','😀']) {
  const text=new TerminalBuffer().push(prefix+'x'.repeat(limit-1)).join('\n');
  assert.ok(text.length<=limit);assert.ok(/^x+$/.test(text));
 }
 const rows=new TerminalBuffer().push('\x1b]0;'+'\n'.repeat(600)+'hidden title\x07Visible');
 assert.equal(rows.join('\n'),'Visible');
});
test('快速显示更新只保留最新待绘制值，不积累 xterm 写入任务',()=>{
 const writes=[],callbacks=[];let resets=0;
 const r=new TerminalRenderer((data,done)=>{writes.push(data);callbacks.push(done);},()=>resets++);
 r.push('first');for(let n=0;n<10000;n++)r.push('first '+n);
 assert.deepEqual(writes,['first']);callbacks.shift()();assert.deepEqual(writes,['first',' 9999']);
 callbacks.shift()();r.push('truncated latest');assert.equal(resets,1);
 r.push('ignored after stop');r.stop();callbacks.shift()();assert.equal(writes.length,3);
});
test('容器输出等待渲染回调，取消会解除等待且保留字节',async()=>{
 const c=new AbortController(),bytes=new Uint8Array([0x1b,0x5b,0x33,0x31,0x6d]);let done;
 let completed=false;const writing=writeTerminal((data,callback)=>{assert.equal(data,bytes);done=callback;},bytes,c.signal).then(()=>completed=true);
 await new Promise(resolve=>setImmediate(resolve));assert.equal(completed,false);done();await writing;assert.equal(completed,true);
 const waiting=writeTerminal(()=>{},bytes,c.signal);c.abort(new Error('窗口已关闭'));
 await assert.rejects(waiting,/窗口已关闭/);
 await assert.rejects(writeTerminal(()=>assert.fail('不得写入'),bytes,c.signal),/窗口已关闭/);
 const active=new AbortController();await assert.rejects(writeTerminal(()=>{throw new Error('终端已释放');},bytes,active.signal),/终端已释放/);
});
