import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mergeStackRefresh, mergeStackListRefresh } from '../src/lib/stack-refresh.ts';
const base = {id:'test',name:'test',compose:'saved',savedCompose:'saved',env:'A=1',savedEnv:'A=1',status:'down',busy:false,deployLog:['real output'],managed:true,loadError:''};
test('刷新完成时保留请求期间编辑的草稿', () => {
 const current = {...base,compose:'unsaved'};
 const result = mergeStackRefresh({...base,status:'up'},current,base);
 assert.equal(result.compose,'unsaved'); assert.equal(result.status,'up');
 assert.deepEqual(result.deployLog,['real output']);
});
test('保存在刷新请求期间完成时不能用旧响应回滚', () => {
 const current = {...base,compose:'new saved',savedCompose:'new saved'};
 assert.equal(mergeStackRefresh(base,current,base).compose,'new saved');
});
test('执行中保留草稿、状态和输出', () => {
 const current = {...base,busy:true,deployLog:['new runtime output']};
 const result = mergeStackRefresh({...base,compose:'server'},current,base);
 assert.equal(result.busy,true); assert.equal(result.compose,'saved'); assert.deepEqual(result.deployLog,current.deployLog);
});
test('非编辑状态接受最新文件但保留真实执行记录', () => {
 assert.equal(mergeStackRefresh({...base,compose:'server'},base,base).compose,'server');
});

test('运行状态查询失败保留错误明细，不变成文件读取失败', () => {
 const result = mergeStackRefresh({...base,status:'unknown',runtimeError:'unsupported_field is not allowed'},base,base);
 assert.equal(result.compose,base.compose);
 assert.equal(result.managed,true);
 assert.equal(result.loadError,'');
 assert.equal(result.runtimeError,'unsupported_field is not allowed');
 assert.equal(result.status,'unknown');
 assert.equal(mergeStackRefresh({...base,runtimeError:''},result,result).runtimeError,'');
});

test('仅 .env 未保存修改也必须保留，保存基线不会被推送覆盖', () => {
 const current = {...base,env:'A=changed'};
 const result = mergeStackRefresh({...base,env:'A=server',savedEnv:'A=server'},current);
 assert.equal(result.env,'A=changed'); assert.equal(result.savedEnv,'A=1');
});

test('整批刷新按 ID 合并，保留 YAML、env、执行状态和错误，移除已删除栈', () => {
 const current = [{...base,id:'yaml',compose:'draft'},{...base,id:'env',env:'A=draft'},{...base,id:'busy',busy:true,deployLog:['running output']},{...base,id:'deleted'}];
 const incoming = ['busy','yaml','env','new'].map(id => ({...base,id,status:'up',compose:'server',savedCompose:'server',env:'A=server',savedEnv:'A=server',runtimeError:'runtime detail'}));
 const result = mergeStackListRefresh(incoming,current);
 assert.deepEqual(result.map(s=>s.id),['busy','yaml','env','new']);
 assert.equal(result[0].busy,true); assert.equal(result[0].deployLog,current[2].deployLog);
 assert.equal(result[1].compose,'draft'); assert.equal(result[1].savedCompose,'saved');
 assert.equal(result[2].env,'A=draft'); assert.equal(result[2].savedEnv,'A=1');
 assert.equal(result[3],incoming[3]);
 assert.ok(result.every(s=>s.status==='up' && s.runtimeError==='runtime detail'));
 assert.equal(current[0].compose,'draft'); assert.equal(incoming[1].compose,'server');
 assert.deepEqual(mergeStackListRefresh([],current),[]);
});

test('批量合并只线性读取旧列表，不为每个新栈重新扫描', () => {
 const size = 1000;
 let idReads=0;
 const current=Array.from({length:size},(_,i)=>({...base,get id(){ idReads++; return String(i); }}));
 const incoming=Array.from({length:size},(_,i)=>({...base,id:String(size-1-i)}));
 const result=mergeStackListRefresh(incoming,current);
 assert.equal(result.length,size);
 assert.ok(idReads<=size*2,`旧列表 ID 被读取 ${idReads} 次，应为线性数量`);
});

test('重复旧 ID 保持原 find 的首项匹配语义', () => {
 const first={...base,compose:'first draft'},second={...base,compose:'second draft'};
 assert.equal(mergeStackListRefresh([base],[first,second])[0].compose,'first draft');
});
