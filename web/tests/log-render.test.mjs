import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {createRoot,mapArray} from '../node_modules/solid-js/dist/solid.js';
import {createStore,reconcile} from '../node_modules/solid-js/store/dist/store.js';
const frameCode=stripTypeScriptTypes(readFileSync(new URL('../src/lib/frame-update.ts',import.meta.url),'utf8'),{mode:'transform'});
const {FrameUpdate}=await import('data:text/javascript;base64,'+Buffer.from(frameCode).toString('base64'));
import {LogBuffer} from '../src/lib/log-buffer.ts';
import {LogReplay} from '../src/lib/log-replay.ts';

function frames() {
 let id=0;const pending=new Map();
 return {pending,request:cb=>{pending.set(++id,cb);return id;},cancel:id=>pending.delete(id),draw(){const callbacks=[...pending.values()];pending.clear();callbacks.forEach(cb=>cb(0));}};
}

test('每帧只绘制一次，结束即时绘制末帧，清空撤销排队工作',()=>{
 const queue=frames();let renders=0;
 const drawing=new FrameUpdate(()=>renders++,queue.request,queue.cancel);
 for(let i=0;i<10000;i++)drawing.schedule();
 assert.equal(queue.pending.size,1);assert.equal(renders,0);
 queue.draw();assert.equal(renders,1);
 drawing.schedule();drawing.flush();assert.equal(renders,2);assert.equal(queue.pending.size,0);
 drawing.flush();assert.equal(renders,2);
 drawing.schedule();drawing.cancel();queue.draw();assert.equal(renders,2);
 drawing.schedule();queue.draw();assert.equal(renders,3);
});

test('行 ID 在分片、进度覆盖和换行之间稳定，清空不复用旧行 ID',()=>{
 const buffer=new LogBuffer();buffer.push('Down');const id=buffer.snapshot()[0].id;
 buffer.push('loading');assert.equal(buffer.snapshot()[0].id,id);
 buffer.push('\r\x1b[2KDone\nnext');const rows=buffer.snapshot();
 assert.deepEqual(rows.map(r=>r.raw),['Done','next']);assert.equal(rows[0].id,id);assert.notEqual(rows[1].id,id);
 buffer.clear();buffer.push('new\n');assert.notEqual(buffer.snapshot()[0].id,id);
});

const source=readFileSync(new URL('../src/store.ts',import.meta.url),'utf8');
const openCode=stripTypeScriptTypes(source.slice(source.indexOf('export function openLog('),source.indexOf('export function stopLog(')).replace('export function','function'),{mode:'transform'});
function fixture() {
 const queue=frames();let receive,scrolls=0,mounts=0;
 const [app,setApp]=createStore({stacks:[],logView:{lines:[],connection:'ended'}});
 const rendered=mapArray(()=>app.logView.lines,row=>{mounts++;return row;});
 const result=new Function('app','setApp','reconcile','LogBuffer','LogReplay','FrameUpdate','DemandStream','events','terminalPath','logRequested','setPausedLog','scrollLogEnd',`
 let logStream=null,resetLogBuffer=()=>{};const stopLog=()=>{logStream?.stop();resetLogBuffer();};
 ${openCode}
 return {openLog,clear:()=>{resetLogBuffer();setApp('logView','lines',[]);},stopLog};`)(
 app,setApp,reconcile,LogBuffer,LogReplay,class extends FrameUpdate {constructor(render){super(render,queue.request,queue.cancel);}},
 class {constructor(options){this.options=options;}start(){this.controller=new AbortController();void this.options.connect(this.controller.signal);}stop(){this.controller.abort();}},
 (_path,callback)=>{receive=callback;return Promise.resolve();},()=>'/terminal/test/container-logs',()=>true,()=>{},()=>scrolls++);
 result.openLog('logs','test');
 return {...result,queue,app,rendered,mounts:()=>mounts,scrolls:()=>scrolls,append:text=>receive({event:'log',data:{text,time:''}})};
}

test('实际日志入口批量收取不重复建行；暂停快照、清空和关闭没有晚到绘制',()=>{
 createRoot(dispose=>{
  const f=fixture();
  for(let i=0;i<500;i++)f.append(`line ${i}\n`);
  assert.equal(f.queue.pending.size,1);assert.equal(f.app.logView.lines.length,0);
  f.queue.draw();f.rendered();const first=f.app.logView.lines[0];
  assert.equal(f.mounts(),500);assert.equal(f.scrolls(),1);
  f.append('partial');f.queue.draw();f.rendered();
  assert.equal(f.app.logView.lines.length,500);assert.equal(f.mounts(),501);
  const tail=f.app.logView.lines.at(-1),paused=f.app.logView.lines.map(row=>({...row}));
  f.append(' completed\n');f.queue.draw();f.rendered();
  assert.equal(f.app.logView.lines.at(-1),tail);assert.equal(tail.raw,'partial completed');assert.equal(f.mounts(),501);
  assert.equal(paused.at(-1).raw,'partial');assert.equal(first.raw,'line 0');
  f.append('queued');f.clear();f.queue.draw();assert.equal(f.app.logView.lines.length,0);
  f.append('new');f.queue.draw();assert.equal(f.app.logView.lines[0].raw,'new');
  f.append('must not render after close');f.stopLog();f.queue.draw();assert.equal(f.app.logView.lines[0].raw,'new');
  dispose();
 });
});
