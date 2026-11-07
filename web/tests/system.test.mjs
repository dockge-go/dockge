import { test } from 'node:test';
import assert from 'node:assert/strict';
import { percentText, uptimeParts, systemInfoRows, resourceKnown } from '../src/lib/system.ts';

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
