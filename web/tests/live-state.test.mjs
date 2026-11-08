import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
const source = stripTypeScriptTypes(readFileSync(new URL('../src/lib/live-state.ts', import.meta.url), 'utf8'), {mode:'transform'});
const {LiveState} = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => {resolve=yes; reject=no;}); return {promise,resolve,reject}; };
const tick = () => new Promise(resolve => setImmediate(resolve));
function fixture(t, options={}) {
 const connections=[], requests=[], received=[], failures=[];
 const state = new LiveState({active:()=>true, reconnectDelay:5,
  connect(receive,signal) {const done=deferred(); connections.push({receive,signal,...done}); return done.promise;},
  requestRefresh(signal) {const done=deferred(); requests.push({signal,...done}); return done.promise;},
  receive:event=>received.push(event), disconnected:error=>failures.push(error), ...options,
 });
 t.after(()=>state.stop());
 const snapshot = (revision, value=revision) => connections.at(-1).receive({event:'snapshot',data:{revision,value}});
 return {state,connections,requests,received,failures,snapshot};
}
test('重复刷新复用唯一 SSE 和唯一在途刷新请求',async t=>{
 const f=fixture(t), first=f.state.refresh();
 assert.equal(f.state.refresh(),first); assert.equal(f.connections.length,1);
 f.snapshot(1); await first;
 for(let revision=2;revision<50;revision++) {
  const refresh=f.state.refresh(); assert.equal(f.state.refresh(),refresh);
  f.requests.at(-1).resolve({revision}); await tick(); f.snapshot(revision); await refresh;
 }
 assert.equal(f.connections.length,1); assert.equal(f.connections[0].signal.aborted,false);
});
test('刷新确认之前与边界之前的旧帧不能覆盖状态或完成刷新',async t=>{
 const f=fixture(t), first=f.state.refresh(); f.snapshot(1); await first;
 const refresh=f.state.refresh(); let settled=false; refresh.then(()=>settled=true);
 f.snapshot(1,'old'); f.snapshot(2,'before HTTP ack');
 assert.equal(f.received.length,1);
 f.requests[0].resolve({revision:2}); await tick();
 f.snapshot(1,'late old frame'); await tick(); assert.equal(settled,false);
 f.snapshot(2,'fresh'); await refresh;
 assert.equal(f.received.length,2); assert.equal(f.received.at(-1).data.value,'fresh');
});
test('停止会取消 SSE 与刷新，旧连接回调不能影响新会话',async t=>{
 const f=fixture(t), first=f.state.refresh(); f.snapshot(1); await first;
 const refresh=f.state.refresh(), old=f.connections[0];
 f.state.stop(); await refresh;
 assert.equal(old.signal.aborted,true); assert.equal(f.requests[0].signal.aborted,true);
 const fresh=f.state.refresh(); f.snapshot(10); await fresh;
 old.receive({event:'snapshot',data:{revision:999}}); old.resolve();
 f.requests[0].resolve({revision:999}); await tick();
 assert.equal(f.received.length,2); assert.equal(f.failures.length,0);
 assert.equal(f.connections[1].signal.aborted,false);
});
test('EOF 只安排一次重连，手动刷新取消待执行重连',async t=>{
 const f=fixture(t), first=f.state.refresh(); f.snapshot(1); await first;
 f.connections[0].resolve(); await tick();
 const refresh=f.state.refresh(); f.snapshot(2); await refresh;
 await new Promise(resolve=>setTimeout(resolve,15));
 assert.equal(f.connections.length,2); assert.equal(f.failures.length,1);
});
test('后台或注销状态不建立连接，失败重连也服从活跃状态',async t=>{
 let active=false; const f=fixture(t,{active:()=>active});
 await f.state.refresh(); assert.equal(f.connections.length,0);
 active=true; const first=f.state.refresh();
 active=false; f.connections[0].reject(new Error('真实网络失败')); await first; await tick();
 assert.equal(f.failures[0].message,'真实网络失败');
 await new Promise(resolve=>setTimeout(resolve,15)); assert.equal(f.connections.length,1);
});
test('刷新请求失败不重建 SSE，错误保留原文且后续状态仍可更新',async t=>{
 const f=fixture(t), first=f.state.refresh(); f.snapshot(1); await first;
 const refresh=f.state.refresh(); f.requests[0].reject(new Error('HTTP 401: 会话已到期')); await refresh;
 assert.equal(f.failures[0].message,'HTTP 401: 会话已到期');
 f.snapshot(2); assert.equal(f.received.length,2); assert.equal(f.connections.length,1);
});
test('操作完成刷新不能复用操作开始前的在途采样或刷新确认',async t=>{
 const f=fixture(t), initial=f.state.refresh();
 const afterSave=f.state.refresh(true);
 assert.equal(afterSave,initial); assert.equal(f.requests.length,1);
 f.snapshot(1,'before save'); assert.equal(f.received.length,0);
 f.requests[0].resolve({revision:2}); await tick(); f.snapshot(2,'saved'); await afterSave;
 const beforeDelete=f.state.refresh(), afterDelete=f.state.refresh(true);
 assert.equal(beforeDelete,afterDelete); assert.equal(f.requests.length,3);
 // 后发的删除刷新先返回，之前的刷新确认不能把边界倒退。
 f.requests[2].resolve({revision:4}); await tick();
 f.requests[1].resolve({revision:3}); await tick();
 f.snapshot(3,'before delete'); assert.equal(f.received.length,1);
 f.snapshot(4,'deleted'); await afterDelete;
 assert.equal(f.received.at(-1).data.value,'deleted'); assert.equal(f.connections.length,1);
});
test('无效的刷新确认不能把旧数据当作最新',async t=>{
 const f=fixture(t), first=f.state.refresh(); f.snapshot(1); await first;
 const refresh=f.state.refresh(); f.requests[0].resolve({}); await refresh; await tick();
 assert.match(f.failures[0].message,/revision/); assert.equal(f.connections[0].signal.aborted,true);
});

test('空刷新确认会关闭连接，不产生未处理异常',async t=>{
 for(const result of [null,undefined,{revision:-1},{revision:Infinity},{revision:'1'}]) {
  const f=fixture(t), initial=f.state.refresh(); f.snapshot(1); await initial;
  const refresh=f.state.refresh(); f.requests[0].resolve(result); await refresh; await tick();
  assert.match(f.failures[0].message,/revision/); assert.equal(f.connections[0].signal.aborted,true);
  f.state.stop();
 }
});
test('连接与刷新同步抛错也能回收状态并保留明细',async t=>{
 const f=fixture(t,{connect(){throw new Error('连接无法创建');}});
 await f.state.refresh(); assert.equal(f.failures[0].message,'连接无法创建'); f.state.stop();
 const g=fixture(t,{requestRefresh(){throw new Error('刷新请求无法创建');}});
 const initial=g.state.refresh(); g.snapshot(1); await initial;
 await g.state.refresh(); assert.equal(g.failures[0].message,'刷新请求无法创建');
 assert.equal(g.connections.length,1);
});

test('刷新失败也不能把已显示的新状态倒退为旧 revision',async t=>{
 const f=fixture(t),initial=f.state.refresh();f.snapshot(2);await initial;
 f.snapshot(1,'old');assert.equal(f.received.length,1);
 const refresh=f.state.refresh();f.requests[0].reject(new Error('刷新网络失败'));await refresh;
 f.snapshot(1,'late old');assert.equal(f.received.length,1);
 f.snapshot(3,'latest');assert.equal(f.received.at(-1).data.value,'latest');
});
