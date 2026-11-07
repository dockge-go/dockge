import { test } from 'node:test';
import assert from 'node:assert/strict';
import { percentText, uptimeParts, systemInfoRows, startSystemPolling, resourceKnown } from '../src/lib/system.ts';

test('未知值不能变成 0%，合法真实 0 必须显示',()=>{
 for(const value of [null,undefined,NaN,Infinity,-1,101]) assert.equal(percentText(value),null);
 assert.equal(percentText(0),'0.0%');assert.equal(percentText(42.14),'42.1%');assert.equal(percentText(100),'100.0%');
 assert.equal(uptimeParts(null),null);assert.deepEqual(uptimeParts(90061),{days:1,hours:1,minutes:1});
});
test('系统信息仅展示白名单，不泄露零散原始属性或伪造主机信息',()=>{
 const info={ServerVersion:'28.0.0',Name:'container-hostname',Debug:'true',MemTotal:'12345',Secret:'must-not-show'};
 const rows=systemInfoRows(null,info,'docker');
 assert.equal(rows.length,9);assert.equal(rows.find(row=>row[0]==='sysinfo.hostname')[1],null);
 assert.equal(rows.find(row=>row[0]==='sysinfo.totalMemory')[1],null);
 assert.ok(!JSON.stringify(rows).includes('must-not-show'));assert.ok(!JSON.stringify(rows).includes('container-hostname'));
 assert.equal(rows.find(row=>row[0]==='sysinfo.runtimeVersion')[1],'28.0.0');
});
test('运行时资源加载或失败时数量未知，成功空列表才显示真实零',()=>{
 assert.equal(resourceKnown(true,{},'containers'),false);
 for(const key of ['request','resources','containers']) assert.equal(resourceKnown(false,{[key]:'实际 CLI 失败原因'},'containers'),false);
 assert.equal(resourceKnown(false,{},'containers'),true);
 assert.equal(resourceKnown(false,{images:'镜像查询失败'},'containers'),true);
 assert.equal(resourceKnown(false,{images:'镜像查询失败'},'images'),false);
});
const flush=async()=>{for(let i=0;i<5;i++) await Promise.resolve();};
test('每 2 秒轻量刷新，请求不重叠，停止后不写回',async t=>{
 t.mock.timers.enable({apis:['setInterval','setTimeout']});
 let requests=0,success=0,resolve; const signals=[];
 const poll=startSystemPolling({session:()=> 'session-a',visible:()=>true,request:signal=>{signals.push(signal);requests++;return new Promise(r=>{resolve=r;});},success:()=>success++,failure:()=>{}});
 assert.equal(requests,1);t.mock.timers.tick(4000);assert.equal(requests,1);
 resolve({cpu:30});await flush();assert.equal(success,1);
 t.mock.timers.tick(2000);assert.equal(requests,2);
 poll.stop();assert.equal(signals[1].aborted,true);resolve({cpu:50});await flush();assert.equal(success,1);
 t.mock.timers.tick(10000);assert.equal(requests,2);
});
test('隐藏时暂停，恢复可见自动重连；会话切换丢弃旧响应',async t=>{
 t.mock.timers.enable({apis:['setInterval','setTimeout']});
 let visible=false,session='a',requests=0,success=0,resolve;
 const poll=startSystemPolling({session:()=>session,visible:()=>visible,request:()=>{requests++;return new Promise(r=>{resolve=r;});},success:()=>success++,failure:()=>{}});
 t.mock.timers.tick(4000);assert.equal(requests,0);
 visible=true;poll.visibilityChanged();assert.equal(requests,1);
 session='b';resolve(1);await flush();assert.equal(success,0);
 t.mock.timers.tick(2000);assert.equal(requests,2);
 visible=false;poll.visibilityChanged();resolve(2);await flush();assert.equal(success,0);
 t.mock.timers.tick(4000);assert.equal(requests,2);
 visible=true;poll.visibilityChanged();resolve(3);await flush();assert.equal(success,1);
 poll.stop();
});
test('请求超时保留具体原因而非无限显示旧值',async t=>{
 t.mock.timers.enable({apis:['setInterval','setTimeout']});
 let error;
 const poll=startSystemPolling({session:()=> 'a',visible:()=>true,request:signal=>new Promise((_,reject)=>signal.addEventListener('abort',()=>reject(signal.reason))),success:()=>{assert.fail('timed out request cannot succeed');},failure:e=>{error=e;}});
 t.mock.timers.tick(5000);await flush();assert.match(error.message,/system.*5000ms/);poll.stop();
});
