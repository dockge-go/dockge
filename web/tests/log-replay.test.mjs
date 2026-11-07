import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const {LogReplay}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(readFileSync(new URL('../src/lib/log-replay.ts',import.meta.url),'utf8'))).toString('base64'));
const row=(text,time='2026-11-08T00:00:00.000000000Z')=>({text,time});
test('重连按真实出现次数去重，同一时间戳同文案新增仍保留',()=>{
 const replay=new LogReplay(), record=row('重复\n');
 assert.equal(replay.push(record),'重复\n'); assert.equal(replay.push(record),'重复\n');
 assert.equal(replay.begin(),record.time);
 assert.equal(replay.push(record),''); assert.equal(replay.push(record),''); assert.equal(replay.push(record),'重复\n');
 replay.begin(); assert.equal(replay.push(record),''); assert.equal(replay.push(record),''); assert.equal(replay.push(record),'');
 assert.equal(replay.push(row('断线期间新日志\n','2026-11-08T00:00:01.000000000Z')),'断线期间新日志\n');
});
test('多服务乱序取最早游标，未知时间不吞日志，历史有界',()=>{
 const replay=new LogReplay(); assert.equal(replay.begin(),undefined);
 replay.push(row('new','2026-11-08T00:00:02.000000000Z')); replay.push(row('old'));
 assert.equal(replay.begin(),'2026-11-08T00:00:00.000000000Z');
 const unknown=row('原始stderr',''); assert.equal(replay.push(unknown),unknown.text); replay.begin(); assert.equal(replay.push(unknown),unknown.text);
 for(let i=0;i<5001;i++)replay.push(row(String(i),'2026-11-08T00:00:03.000000000Z'));
 assert.equal(replay.begin(),'2026-11-08T00:00:03.000000000Z');
});
