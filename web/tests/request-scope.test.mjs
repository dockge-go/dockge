import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { RequestScope } from '../src/lib/request-scope.ts';
const deferred=()=>{let resolve,reject;const promise=new Promise((r,j)=>{resolve=r;reject=j;});return {promise,resolve,reject};};

test('注销中止全部在途请求，旧响应即使晚到也不再完成操作，新会话可正常请求',async()=>{
 const scope=new RequestScope(),a=deferred(),b=deferred();let signalA,signalB;
 const first=scope.run(signal=>{signalA=signal;return a.promise;});
 const second=scope.run(signal=>{signalB=signal;return b.promise;});
 scope.cancel();assert.equal(signalA.aborted,true);assert.equal(signalB.aborted,true);
 a.resolve('旧响应');b.resolve('旧响应');
 await assert.rejects(first,{name:'AbortError'});await assert.rejects(second,{name:'AbortError'});
 assert.equal(await scope.run(async()=> '新会话'),'新会话');
});
test('按需面板单独关闭不取消其他请求，完成后移除外部 signal 监听器',async()=>{
 const scope=new RequestScope(),external=new AbortController(),d=deferred();let signal;
 const pending=scope.run(s=>{signal=s;return d.promise;},external.signal);
 external.abort();assert.equal(signal.aborted,true);
 assert.equal(await scope.run(async()=>42),42);d.resolve(1);
 await assert.rejects(pending,{name:'AbortError'});
 let finished;
 const other=new AbortController();
 await scope.run(async s=>{finished=s;},other.signal);
 other.abort();scope.cancel();assert.equal(finished.aborted,false);
});

test('已取消的外部请求不会运行，真实业务错误保持原文',async()=>{
 const scope=new RequestScope(),c=new AbortController();c.abort();let calls=0;
 await assert.rejects(scope.run(async()=>{calls++;},c.signal),{name:'AbortError'});assert.equal(calls,0);
 await assert.rejects(scope.run(async()=>{throw new Error('镜像仍被容器引用');}),/镜像仍被容器引用/);
});

const source=readFileSync(new URL('../src/store.ts',import.meta.url),'utf8');
const code=stripTypeScriptTypes(source.slice(source.indexOf('async function removeBatch('),source.indexOf('export function clearStopped(')),{mode:'transform'});
function batchFixture() {
 let token='old';const calls=[],toasts=[],d=deferred();
 const removeBatch=new Function('AuthAPI','action','t','toast',code+'\nreturn removeBatch;')(
  {token:()=>token},async(kind,id)=>{calls.push(id);if(id==='first')await d.promise;},key=>key,(...args)=>toasts.push(args)
 );
 return {calls,toasts,d,removeBatch,changeToken:value=>{token=value;}};
}
for(const outcome of ['resolve','reject'])test('会话切换后批量删除停止，不携带新 token 删除下一项：'+outcome,async()=>{
 const f=batchFixture(),pending=f.removeBatch('images',['first','second']);
 f.changeToken('new');f.d[outcome](outcome==='reject'?new Error('旧请求已取消'):undefined);
 await assert.rejects(pending,{name:'AbortError'});assert.deepEqual(f.calls,['first']);assert.deepEqual(f.toasts,[]);
});
test('正常批量操作会完整执行并汇总结果',async()=>{
 const f=batchFixture(),pending=f.removeBatch('images',['first','second']);f.d.resolve();await pending;
 assert.deepEqual(f.calls,['first','second']);assert.equal(f.toasts.length,1);
});
