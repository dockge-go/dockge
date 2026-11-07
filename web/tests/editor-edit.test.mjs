import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source = stripTypeScriptTypes(readFileSync(new URL('../src/lib/editor-edit.ts', import.meta.url), 'utf8'));
const {indentEdit} = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
const apply = (v,e) => v.slice(0,e.from)+e.text+v.slice(e.to);
test('Tab 插入两空格，Shift+Tab 删除行首缩进并保持光标',()=>{
 let v='services:\n  app:\n';const e=indentEdit(v,v.length,v.length,false);v=apply(v,e);
 const undo=indentEdit(v,e.start,e.end,true);assert.equal(apply(v,undo),'services:\n  app:\n');assert.equal(undo.start,17);
});
test('多行选择缩进和反缩进可还原原文及选区',()=>{
 const v='services:\n  a:\n  b:\nother: x';const start=10,end=19;
 const e=indentEdit(v,start,end,false);const x=apply(v,e);const back=indentEdit(x,e.start,e.end,true);
 assert.equal(apply(x,back),v);assert.equal(back.start,start);assert.equal(back.end,end);
});
test('无缩进行 Shift+Tab 不删除正文，行首选区不包含下一行',()=>{
 const v='app:\nnext:';const e=indentEdit(v,0,5,false);assert.equal(apply(v,e),'  app:\nnext:');
 const back=indentEdit(v,2,2,true);assert.equal(apply(v,back),v);assert.equal(back.start,2);
});
