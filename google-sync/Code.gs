/** Standalone Apps Script. Do not share this script with spreadsheet editors.
 * Script properties: API_BASE, API_TOKEN, CAMP_SHEET_ID, TRES_SHEET_ID.
 * Run install once as the owner, after database migration and API deployment.
 */
function config_() {
  const p = PropertiesService.getScriptProperties().getProperties();
  p.API_BASE = p.API_BASE || 'https://agape-camp.onrender.com';
  p.CAMP_SHEET_ID = p.CAMP_SHEET_ID || '14S7kWQwNyVa2mLR_Pvw03Aw0I_dscdouOF5-K8nEZWE';
  p.TRES_SHEET_ID = p.TRES_SHEET_ID || '1dQ7sXYPE8i4fUhgITtnp4CeQq2RLhSvkoLJF1Z6XCzk';
  if (!/^https:\/\//.test(p.API_BASE || '') || (p.API_TOKEN || '').length < 32 || !p.CAMP_SHEET_ID || !p.TRES_SHEET_ID) throw new Error('Сначала заполни настройки подключения');
  return p;
}
function api_(kind, edit) {
  const p = config_();
  const options = {method: edit ? 'post' : 'get', headers: {Authorization: 'Bearer ' + p.API_TOKEN}, muteHttpExceptions: true, followRedirects: false};
  if (edit) { options.contentType = 'application/json'; options.payload = JSON.stringify(edit); }
  const response = UrlFetchApp.fetch(p.API_BASE + '/api/sheets-sync?kind=' + kind, options);
  let body; try { body = JSON.parse(response.getContentText()); } catch (_) { throw new Error('Сервер временно недоступен'); }
  return {status: response.getResponseCode(), body};
}
function targets_() { const p=config_(); return [{kind:'camp',id:p.CAMP_SHEET_ID},{kind:'tres',id:p.TRES_SHEET_ID}]; }
function text_(v) { const s=String(v == null ? '' : v); return /^[=+@-]/.test(s) ? "'"+s : s; }
function display_(f,v) {
  if (v == null) return '';
  if (f.kind === 'list') return v.join('; ');
  if (f.kind === 'bool') return v ? 'Да' : 'Нет';
  if (f.kind === 'role') return v === 'servant' ? 'Служитель' : 'Участник';
  if (f.kind === 'date') return String(v).replace(/-/g,'.');
  return String(v);
}
function parse_(f,v) {
  const s=String(v).trim();
  if(f.kind==='list') return s ? s.split(';').map(x=>x.trim()).filter(Boolean) : [];
  if(f.kind==='bool') {if(!s)return null;if(s==='Да')return true;if(s==='Нет')return false;throw new Error(f.label+': введи Да или Нет');}
  if(f.kind==='date') return s.replace(/\./g,'-');
  return s;
}
function eq_(a,b) {return JSON.stringify(a)===JSON.stringify(b);}
function status_(sheet,row,col,message) {sheet.getRange(row,col).setValue(text_(message));}
function state_(ss) {
  const sheet=ss.getSheetByName('_sync');
  if(!sheet)throw new Error('Не найден служебный лист: запусти install');
  const map={};if(sheet.getLastRow()>1) sheet.getRange(2,1,sheet.getLastRow()-1,2).getValues().forEach((r,i)=>{if(r[0])map[r[0]]={index:i+2,data:JSON.parse(r[1])};});
  return {sheet,map};
}
function saveState_(st,id,data) {
  let rec=st.map[id];if(!rec){rec={index:st.sheet.getLastRow()+1};st.map[id]=rec;}
  rec.data=data;st.sheet.getRange(rec.index,1,1,2).setValues([[id,JSON.stringify(data)]]);
}
function journal_(ss,id,message,details) {
  ss.getSheetByName('История').appendRow([new Date().toISOString(),id,text_(message),text_(JSON.stringify(details))]);
}
function install() {
  targets_().forEach(t=>{
    const response=api_(t.kind);if(response.status!==200)throw new Error('Нет доступа к серверу: '+response.status);
    const ss=SpreadsheetApp.openById(t.id), fields=response.body.fields;
    const sheet=ss.getSheetByName('Анкеты');if(!sheet)throw new Error('Нет листа Анкеты');
    sheet.getRange(1,1,1,fields.length+1).setValues([[...fields.map(f=>f.label),'Синхронизация']]);
    sheet.setFrozenRows(1);sheet.setFrozenColumns(2);
    if(!sheet.getFilter())sheet.getRange(1,1,sheet.getMaxRows(),fields.length+1).createFilter();
    sheet.getRange(1,1,1,fields.length+1).setBackground('#eeeeee').setFontWeight('bold').setWrap(true);
    sheet.setRowHeight(1,52);
    let state=ss.getSheetByName('_sync');if(!state){state=ss.insertSheet('_sync');state.appendRow(['ID','Последнее согласованное состояние']);state.hideSheet();}
    if(!ss.getSheetByName('История')){ss.insertSheet('История').appendRow(['Время','ID','Событие','Подробности']);}
    [sheet,state,ss.getSheetByName('История')].forEach(s=>{
      let protection=s.getProtections(SpreadsheetApp.ProtectionType.SHEET).find(p=>p.getDescription()==='AGAPE sync');
      if(!protection)protection=s.protect().setDescription('AGAPE sync');
      const owner=Session.getEffectiveUser();protection.addEditor(owner);protection.removeEditors(protection.getEditors().filter(u=>u.getEmail()!==owner.getEmail()));if(protection.canDomainEdit())protection.setDomainEdit(false);
      if(s===sheet)protection.setUnprotectedRanges(fields.flatMap((f,i)=>f.editable?[sheet.getRange(2,i+1,sheet.getMaxRows()-1,1)]:[]));
    });
  });
  // Replace only this project's own bridge triggers, never unrelated ones.
  ScriptApp.getProjectTriggers().filter(t=>['syncAll','captureEdit'].includes(t.getHandlerFunction())).forEach(t=>ScriptApp.deleteTrigger(t));
  ScriptApp.newTrigger('syncAll').timeBased().everyMinutes(1).create();
  targets_().forEach(t=>ScriptApp.newTrigger('captureEdit').forSpreadsheet(t.id).onEdit().create());
  syncAll();
}
function syncAll(){
 const lock=LockService.getScriptLock();if(!lock.tryLock(1000))return;
 try {targets_().forEach(t=>{try{reconcile_(t);}catch(e){const ss=SpreadsheetApp.openById(t.id);ss.getSheetByName('Инструкция').getRange('B2').setValue('Ошибка синхронизации: '+e.message);}});}finally{lock.releaseLock();}
}
function captureEdit(e){
 if(!e || e.range.getSheet().getName()!=='Анкеты' || e.range.getRow()===1)return;
 const t=targets_().find(x=>x.id===e.source.getId());if(!t)return;
 // Preserve the event payload even if reconciliation is busy. Google's edit
 // event supplies old/new values only for a single-cell edit.
 const single=e.range.getNumRows()===1&&e.range.getNumColumns()===1;
 const eventId=e.range.getSheet().getRange(e.range.getRow(),1).getDisplayValue();
 journal_(e.source,eventId,'Правка в Google-таблице',{range:e.range.getA1Notation(),oldValue:single?(e.oldValue||''):null,value:single?(e.value||''):e.range.getDisplayValues()});
 // The scheduled reconciliation also discovers edits if an onEdit trigger fails.
 const lock=LockService.getScriptLock();if(!lock.tryLock(25000))return;
 try{
  const sheet=e.range.getSheet(), st=state_(e.source);
  if(single && st.map[eventId]) {
   const snapshot=api_(t.kind);if(snapshot.status!==200)return;
   const f=snapshot.body.fields[e.range.getColumn()-1];
   if(f && f.editable){
    const edit={id:eventId,requestId:Utilities.getUuid(),base:{},changes:{}};
    try{edit.base[f.key]=parse_(f,e.oldValue||'');edit.changes[f.key]=parse_(f,e.value||'');}catch(_){reconcile_(t);return;}
    const result=api_(t.kind,edit);
    if(result.status===409){
      journal_(e.source,eventId,'Конфликт одновременных правок',{attempt:edit,response:result.body});
      st.map[eventId].data.blocked=true;st.map[eventId].data.base=result.body.row;saveState_(st,eventId,st.map[eventId].data);
      status_(sheet,e.range.getRow(),snapshot.body.fields.length+1,'Конфликт — варианты сохранены на листе История');return;
    }
   }
  }
  for(let row=e.range.getRow();row<=e.range.getLastRow();row++){
   const id=sheet.getRange(row,1).getDisplayValue(),rec=st.map[id];if(!rec)continue;
   // A fresh explicit edit permits retrying a previously rejected row.
   if(rec.data.blocked){rec.data.blocked=false;saveState_(st,id,rec.data);}
  }
  reconcile_(t);
 } finally{lock.releaseLock();}
}
function reconcile_(t){
 const response=api_(t.kind);if(response.status!==200)throw new Error('Сервер ответил '+response.status);
 const ss=SpreadsheetApp.openById(t.id),sheet=ss.getSheetByName('Анкеты'),fields=response.body.fields,st=state_(ss),statusCol=fields.length+1;
 const headers=sheet.getRange(1,1,1,statusCol).getDisplayValues()[0];if(!eq_(headers,[...fields.map(f=>f.label),'Синхронизация']))throw new Error('Заголовки изменены. Восстанови их перед обновлением');
 const rows=sheet.getLastRow()>1?sheet.getRange(2,1,sheet.getLastRow()-1,statusCol).getDisplayValues():[];
 const indexes={};rows.forEach((r,i)=>{if(!r[0])throw new Error('Строка без ID: восстанови удалённый ID');if(indexes[r[0]])throw new Error('Повторяется ID');indexes[r[0]]=i+2;});
 const remoteIds=new Set(response.body.rows.map(r=>r.id));
 for(let remote of response.body.rows){
  const id=remote.id;let row=indexes[id];
  if(!row){row=sheet.getLastRow()+1;if(row>sheet.getMaxRows())sheet.insertRowsAfter(sheet.getMaxRows(),100);sheet.getRange(row,1,1,statusCol).setNumberFormat('@').setValues([[...fields.map(f=>text_(display_(f,remote[f.key]))),'Сохранено']]);saveState_(st,id,{base:remote,blocked:false});continue;}
  const rec=st.map[id];if(!rec){status_(sheet,row,statusCol,'Нет исходной версии — нужна проверка');continue;}
  if(rec.data.blocked)continue;
  let snapshot=sheet.getRange(row,1,1,fields.length).getDisplayValues()[0],changes={},base=rec.data.base;
  try{fields.forEach((f,i)=>{if(f.editable&&snapshot[i]!==display_(f,base[f.key]))changes[f.key]=parse_(f,snapshot[i]);});}
  catch(e){status_(sheet,row,statusCol,e.message);continue;}
  if(Object.keys(changes).length){
   // Persist the exact request before sending: timeout retries reuse its ID.
   const signature=JSON.stringify(changes);
   if(!rec.data.pending||rec.data.pending.signature!==signature){rec.data.pending={signature,edit:{id,requestId:Utilities.getUuid(),base,changes}};saveState_(st,id,rec.data);}
   const result=api_(t.kind,rec.data.pending.edit);
   if(result.status===409 || result.status===422 || result.status===404){
    journal_(ss,id,result.body.message,{attempt:rec.data.pending.edit,response:result.body});
    // Keep the proposed cells visible; next explicit edit retries against the latest base.
    rec.data.base=result.body.row||remote;rec.data.pending=null;rec.data.blocked=true;saveState_(st,id,rec.data);
    status_(sheet,row,statusCol,result.body.message+' — см. История');continue;
   }
   if(result.status!==200){status_(sheet,row,statusCol,'Ожидает повторной отправки');continue;}
   remote=result.body.row;rec.data.pending=null;
  }
  // Do not overwrite a cell that changed since the snapshot was read.
  const latest=sheet.getRange(row,1,1,fields.length).getDisplayValues()[0];
  if(latest[0]!==id)throw new Error('Строки переместились во время обновления; повторим позже');
  let pending=false;
  fields.forEach((f,i)=>{const wanted=display_(f,remote[f.key]);if(latest[i]!==snapshot[i]){pending=true;return;}if(latest[i]!==wanted)sheet.getRange(row,i+1).setNumberFormat('@').setValue(text_(wanted));});
  rec.data.base=remote;saveState_(st,id,rec.data);status_(sheet,row,statusCol,pending?'Есть новые правки — ожидают отправки':'Сохранено');
 }
 Object.keys(indexes).filter(id=>!remoteIds.has(id)).forEach(id=>status_(sheet,indexes[id],statusCol,'Удалена в админке — строка сохранена для истории'));
 ss.getSheetByName('Инструкция').getRange('B2').setValue('Обновлено: '+Utilities.formatDate(new Date(),'Asia/Seoul','yyyy.MM.dd HH:mm:ss'));
}
