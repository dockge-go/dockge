import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { batch } from '../node_modules/solid-js/dist/solid.js';
import { createStore, reconcile } from '../node_modules/solid-js/store/dist/store.js';
import { resourceKnown } from '../src/lib/system.ts';

// 使用实际快照处理与浏览器 Store，防止把对象合并误当成完整替换。
const source = readFileSync(new URL('../src/store.ts', import.meta.url), 'utf8');
const code = stripTypeScriptTypes(source.slice(source.indexOf('function applySnapshot('), source.indexOf('export function refreshData(')), { mode: 'transform' });
function fixture() {
  const [app, setApp] = createStore({ containers:[], images:[], volumes:[], networks:[], stacks:[], info:{ OldField:'旧运行时属性' }, errors:{request:'正在重连', images:'镜像查询失败'}, loading:true, system:null });
  const live = new Function('app', 'setApp', 'batch', 'reconcile', 'mergeStackListRefresh', 'stackStatus', 't', 'LiveState', code + '\nreturn liveState;')(
    app, setApp, batch, reconcile, incoming => incoming, n => n, key => key, class { constructor(options) { this.options = options; } }
  );
  return { app, setApp, receive:live.options.receive };
}
const snapshot = (errors = {}) => ({ revision:1, resources:{containers:[{Id:'真实容器'}],images:[],volumes:[],networks:[],info:{ServerVersion:'29'},disk:[],runtime:'docker',errors},stacks:[],health:{version:'test',runtime:{cli:'docker',compose:'docker',ready:true,stacks:{path:'/opt/stacks',writable:true}}} });

test('恢复快照删除断线与旧来源错误，数量从未知恢复为实际值', () => {
  const f=fixture(); assert.equal(resourceKnown(f.app.loading,f.app.errors,'containers'),false);
  f.receive({event:'snapshot',data:snapshot()});
  assert.deepEqual({...f.app.errors},{});
  assert.equal(resourceKnown(f.app.loading,f.app.errors,'containers'),true);
  assert.equal(f.app.containers.length,1);
  assert.deepEqual({...f.app.info},{ServerVersion:'29'});
});

test('每轮错误集合仅包含当前故障，恢复不掩盖其他来源的新故障', () => {
  const f=fixture(); f.receive({event:'snapshot',data:snapshot({volumes:'inspect 卷失败'})});
  assert.deepEqual({...f.app.errors},{volumes:'inspect 卷失败'});
  assert.equal(resourceKnown(false,f.app.errors,'containers'),true);
  assert.equal(resourceKnown(false,f.app.errors,'volumes'),false);
  f.receive({event:'snapshot',data:snapshot({networks:'inspect 网络失败'})});
  assert.deepEqual({...f.app.errors},{networks:'inspect 网络失败'});
  assert.equal(resourceKnown(false,f.app.errors,'volumes'),true);
});

test('主机指标恢复时删除旧错误和消失的字段，不遗留旧测量值', () => {
  const f=fixture();
  f.receive({event:'system',data:{errors:{memory:'读取内存失败'},cpuPercent:12,load:[1,2,3],memory:null}});
  f.receive({event:'system',data:{errors:{},cpuPercent:0,memory:{used:4,total:8},load:null}});
  assert.deepEqual({...f.app.system.errors},{});
  assert.equal(f.app.system.cpuPercent,0);
  assert.equal(f.app.system.load,null);
  assert.equal(f.app.system.memory.used,4);
  assert.equal(f.app.systemError,''); assert.ok(f.app.systemReceivedAt>0);
});

test('重新打开操作表单删除上次输入和镜像下载输出，不默认显示旧操作', () => {
  const [app,setApp]=createStore({actionBusy:false,form:{imageRef:'旧镜像',output:'上次下载输出',volumeName:'旧卷'},sheet:{open:false,kind:null,param:null}});
  const openCode=stripTypeScriptTypes(source.slice(source.indexOf('export function openSheet('),source.indexOf('export function closeSheet(')).replace('export function','function'),{mode:'transform'});
  const openSheet=new Function('app','setApp','reconcile','closeSheet',openCode+'\nreturn openSheet;')(app,setApp,reconcile,()=>setApp('sheet',{open:false,kind:null,param:null}));
  openSheet('pullImage');
  assert.deepEqual({...app.form},{}); assert.equal(app.sheet.open,true); assert.equal(app.sheet.kind,'pullImage');
  setApp('form','volumeName','刚刚输入');openSheet('newNetwork');
  assert.deepEqual({...app.form},{}); assert.equal(app.sheet.kind,'newNetwork');
});

test('真实资源快照保持对象身份但覆盖所有最新值，删除字段和资源不遗留',()=>{
 const f=fixture();const data=snapshot();data.resources.containers=[{Id:'a',State:'running',Stats:{CPUPct:1},Health:'healthy'},{Id:'b',State:'exited'}];
 data.resources.images=[{Id:'image',Size:1}];data.resources.volumes=[{Name:'volume',UsedBy:[{id:'a'}]}];data.resources.networks=[{ID:'network',Containers:['a']}];
 f.receive({event:'snapshot',data});
 const [a,b]=f.app.containers, image=f.app.images[0],volume=f.app.volumes[0],network=f.app.networks[0];
 const latest=snapshot();latest.resources.containers=[{Id:'b',State:'running'},{Id:'a',State:'exited',Stats:{CPUPct:0}}];
 latest.resources.images=[{Id:'image',Size:2}];latest.resources.volumes=[{Name:'volume',UsedBy:[]}];latest.resources.networks=[{ID:'network',Containers:[]}];
 f.receive({event:'snapshot',data:latest});
 assert.equal(f.app.containers[0],b);assert.equal(f.app.containers[1],a);assert.equal(a.State,'exited');assert.equal(a.Stats.CPUPct,0);assert.equal(a.Health,undefined);
 assert.equal(f.app.images[0],image);assert.equal(image.Size,2);assert.equal(f.app.volumes[0],volume);assert.equal(volume.UsedBy.length,0);assert.equal(f.app.networks[0],network);assert.equal(network.Containers.length,0);
 const empty=snapshot();empty.resources.containers=[];f.receive({event:'snapshot',data:empty});assert.equal(f.app.containers.length,0);assert.equal(f.app.images.length,0);assert.equal(f.app.volumes.length,0);assert.equal(f.app.networks.length,0);
});
