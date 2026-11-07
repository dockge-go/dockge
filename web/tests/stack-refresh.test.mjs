import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mergeStackRefresh } from '../src/lib/stack-refresh.ts';
const base = {id:'test',name:'test',compose:'saved',savedCompose:'saved',env:'A=1',status:'down',busy:false,deployLog:['real output'],composeDeployed:'deployed',managed:true,loadError:''};
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
