import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
const source = stripTypeScriptTypes(readFileSync(new URL('../src/lib/demand-stream.ts', import.meta.url), 'utf8'), {mode:'transform'});
const {DemandStream} = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
const tick = () => new Promise(resolve => setImmediate(resolve));
const delay = () => new Promise(resolve => setTimeout(resolve, 20));
function fixture(t) {
  let active = false;
  const calls = [], states = [], errors = [];
  const stream = new DemandStream({active:()=>active, reconnectDelay:5,
    connect(signal) { return new Promise((resolve,reject)=>calls.push({signal,resolve,reject})); },
    state:value=>states.push(value), failed:error=>errors.push(error.message)});
  t.after(()=>stream.stop());
  return {stream,calls,states,errors,active:value=>active=value};
}
test('未点击不连接；面板激活后只创建一个连接',async t=>{
  const f=fixture(t); await tick(); assert.equal(f.calls.length,0);
  f.stream.start(); assert.equal(f.calls.length,0);
  f.active(true); f.stream.start(); f.stream.start();
  assert.equal(f.calls.length,1); assert.deepEqual(f.states,['connecting']);
});
test('概览刷新不影响日志，EOF 在当前打开面板内重连',async t=>{
  const f=fixture(t); f.active(true); f.stream.start();
  for(let i=0;i<20;i++) f.stream.start();
  assert.equal(f.calls.length,1); assert.equal(f.calls[0].signal.aborted,false);
  f.calls[0].resolve(); await tick(); await delay();
  assert.equal(f.calls.length,2); assert.equal(f.calls[0].signal.aborted,true);
});
test('关闭、切页、注销或隐藏：取消请求与待重连定时器',async t=>{
  for(const beforeEOF of [true,false]) {
    const f=fixture(t); f.active(true); f.stream.start();
    if(!beforeEOF) {f.calls[0].resolve(); await tick();}
    f.active(false); f.stream.stop(); f.calls[0].resolve(); await tick(); await delay();
    assert.equal(f.calls[0].signal.aborted,true); assert.equal(f.calls.length,1);
    f.stream.start(); assert.equal(f.calls.length,1);
  }
});
test('已关闭面板的旧失败不得污染新面板或产生重连',async t=>{
  const f=fixture(t); f.active(true); f.stream.start(); const old=f.calls[0];
  f.stream.stop(); f.stream.start(); old.reject(new Error('旧请求已取消'));
  await tick(); await delay(); assert.equal(f.calls.length,2); assert.deepEqual(f.errors,[]);
  assert.equal(f.calls[1].signal.aborted,false); assert.deepEqual(f.states,['connecting','connecting']);
});
test('重连前再次检查面板状态；具体错误原文保留',async t=>{
  const f=fixture(t); f.active(true); f.stream.start(); f.calls[0].reject(new Error('container logs: permission denied'));
  await tick(); f.active(false); await delay(); assert.equal(f.calls.length,1);
  assert.deepEqual(f.errors,['container logs: permission denied']);
});
test('同步连接失败也按需重试，可安全停止',async t=>{
  const errors=[]; let calls=0;
  const stream=new DemandStream({active:()=>true,connect(){calls++;throw new Error('连接无法创建');},state:()=>{},failed:e=>errors.push(e.message),reconnectDelay:5});
  t.after(()=>stream.stop()); stream.start(); await tick(); stream.stop(); await delay();
  assert.equal(calls,1); assert.deepEqual(errors,['连接无法创建']);
});
