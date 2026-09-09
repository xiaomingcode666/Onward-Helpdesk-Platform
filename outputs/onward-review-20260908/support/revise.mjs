import fs from 'node:fs/promises';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { FileBlob, SpreadsheetFile } from '@oai/artifact-tool';
import { corrections, evidenceSpecs, evidenceLimits } from './corrections.mjs';

const root = 'D:/work/helpdesk';
const out = path.join(root, 'outputs/onward-review-20260908');
const source = 'D:/桌面/Onward_业务功能对照与实施排期_最终版_20260907.xlsx';
const python = 'C:/Users/HASEE/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe';
const extract = spawnSync(python, ['-X', 'utf8', path.join(out, 'support/extract.py')], { encoding: 'utf8' });
if (extract.status !== 0) throw new Error(extract.stderr);
const data = JSON.parse(extract.stdout);
let wb = await SpreadsheetFile.importXlsx(await FileBlob.load(source));
async function render(sheetName, range, name) {
  const blob = await wb.render({ sheetName, range, scale: 1.5, format: 'png' });
  await fs.writeFile(path.join(out, 'support', name + '.png'), new Uint8Array(await blob.arrayBuffer()));
}
if (process.argv.includes('--inspect')) {
  console.log((await wb.inspect({kind: 'sheet', include: 'id,name', maxChars: 3000})).ndjson);
  await render('排期总览', 'A1:H22', 'before-overview');
  await render('功能完成排期', 'A1:F6', 'before-detail');
  console.log('Source previews saved');
  process.exit(0);
}

const reqs = new Map(data.requirements.map(r => [r.id, r]));
const sectionKeys = {
  '§2.3': ['2.3 目标使用场景'],
  '§2.4': ['2.4 V1 范围', 'V1 必须包含', 'V1 明确不包含'],
  '§7.3': ['7.3 Ticket Projection 状态规则'],
  '§8.2': ['8.2 SSO 集成'],
  '§8.3': ['8.3 Email 集成'],
  '§8.5': ['8.5 外部专家接口'],
  '§8.6': ['8.6 API 原则'],
  '§9.1': ['9.1 Ticket 主流程'],
  '§9.2': ['9.2 外部专家交接流程', 'Handoff 前', '等待期间', 'Handback 后'],
  '§9.3': ['9.3 Major Incident'],
  '§9.5': ['9.5 月度报告流程'],
  '§11.5': ['11.5 可移植性与本地化'],
  '§18.6': ['18.6 调整 Reporting 与数据移交'],
  '§附录B': ['附录 B：标准报告页'],
  '§附录C': ['附录 C：标准 Exit Package'],
};
for (const [id, keys] of Object.entries(sectionKeys)) {
  const entries = keys.flatMap(k => data.sections[k] || []);
  if (!entries.length) throw new Error('Missing source section ' + id);
  reqs.set(id, { id, level: '设计章节（非需求编号）', section: keys.join('；'), text: entries.join('\n') });
}
const readCache = new Map();
async function evidencePath(file, marker) {
  if (!readCache.has(file)) readCache.set(file, (await fs.readFile(path.join(root, file), 'utf8')).split(/\r?\n/));
  const line = readCache.get(file).findIndex(x => x.includes(marker));
  if (line < 0) throw new Error('Missing evidence marker: ' + file + ' / ' + marker);
  return root + '/' + file + ':' + (line + 1);
}
const revive = value => value && typeof value === 'object' && value.date ? new Date(value.date + (value.date.endsWith('Z') ? '' : 'Z')) : value;
const original = data.sheets['功能完成排期'].rows.slice(1).map(row => row.map(revive));
const excerptFilters = {
  'FUN-008': {'§2.3': /Email 或电话/, '§9.1': /^(Receive|Validate|Create\/Acknowledge)/},
  'FUN-004': {'§8.2': /^(oauth2-proxy 验证|GLPI 使用|Logout)/},
  'FUN-015': {'§9.1': /^(Classify|Track)/},
  'FUN-016': {'§7.3': /^(Acknowledged|Assigned)/},
  'FUN-017': {'§2.4': /Self-service Portal/},
  'FUN-018': {'§7.3': /^(Restored|Resolved)/, '§9.1': /^Restore\/Resolve/},
  'FUN-019': {'§9.1': /^Close/},
  'FUN-020': {'§7.3': /^Reopen/},
  'FUN-021': {'§7.3': /^(Resolved|Closure Pending|Closed)/, '§9.1': /^Close/},
  'FUN-022': {'§7.3': /^Reopen/},
  'FUN-031': {'§8.5': /^无论渠道如何/, '§9.2': /^(形成结构化|指明要求)/},
  'FUN-032': {'§9.2': /^(验证是否|不满足时|满足时)/},
  'FUN-033': {'§8.5': /^无论渠道如何/, '§9.2': /^(End-to-end|External Expert OLA|用户 Update|到达 Reminder)/},
};
const rows = [];
const evidenceRows = [];
const expected = {};
const functionRefs = new Map();
const changes = [];
const statusSet = ['已完成', '部分完成', '待完成', '待验证', '待复验'];
for (let i = 0; i < original.length; i++) {
  const old = original[i];
  const id = old[0];
  const change = corrections[id] || {};
  const row = old.slice();
  const refs = change.refs || old[15].split(';').filter(Boolean);
  for (const ref of refs) if (!reqs.has(ref)) throw new Error('Unresolved reference ' + id + ': ' + ref);
  const nature = change.nature || '原文要求（功能拆分）';
  let evidenceKind = '原表陈述，未复核';
  let evidenceText = '原表“当前系统已经有的”记录：' + old[4] + '\n本次未取得此项新的代码/运行验收证据。';
  let proves = '仅保留历史判断供复核，不作为当前实现已通过的证明。';
  let limit = '未执行运行场景，未审计全部实现；状态沿用原表工作判断。原文映射仅表示相关，不代表覆盖整条需求。';
  if (evidenceSpecs[id]) {
    const paths = [];
    for (const [file, marker] of evidenceSpecs[id]) paths.push(await evidencePath(file, marker));
    evidenceText = paths.join('\n');
    evidenceKind = ['FUN-004', 'FUN-008', 'FUN-021', 'FUN-031', 'FUN-037', 'FUN-039', 'FUN-043', 'FUN-054', 'FUN-083', 'FUN-084'].includes(id) ? '静态代码核查' : '代码/测试入口定位';
    proves = change.current || '已定位与本项有关的实现/测试入口；测试源码不等于本次执行结果。';
    limit = evidenceLimits[id];
  }
  if (id === 'FUN-025') {
    evidenceKind = '限定范围静态检索';
    evidenceText = '2026-09-08，检索 D:/work/helpdesk/internal 与企业 tickets/ticket-workbench 页面。关键词：merge.*ticket、ticket.*merge、duplicate.*ticket、ticket.*duplicate、merged_into。';
    proves = '匹配到知识候选去重、请求幂等等代码，未定位两张工单合并业务入口。';
    limit = '不是运行失败结论，也不能排除仓库外或其他命名实现。原表 3 人日仅适合初查，不足以承诺新增实现。';
  }
  row[2] = change.name || old[2];
  row[3] = change.status || (old[3] === '已完成' ? '待复验' : old[3]);
  row[4] = change.current || '原表记录（运行未复核）：' + old[4];
  row[5] = change.gap || (old[3] === '已完成' ? '补可复核的测试记录、环境/版本和验收人；确认原文对应范围。' + (old[5].startsWith('无需') ? '' : old[5]) : old[5]);
  row[7] = row[3] === '待复验' ? '补历史证据并复验' : change.status === '部分完成' ? '补齐已知缺口后验收' : id === 'FUN-025' ? '确认实现位置并重新估算' : old[7];
  if (old[3] === '已完成') { row[6] = '待确认'; row[8] = '回归待排'; }
  row[14] = change.accept || old[14];
  row[15] = refs.join('；');
  const reason = change.reason || '保留原功能拆分，补真实原文摘录、来源级别及证据范围；本次不新增验收通过结论。';
  const history = old[3] === '已完成' ? '原表标为已完成，未附可复核运行证据，现标待复验；不是断言原功能失效。' : '原表状态：' + old[3] + '。';
  row[18] = [old[18], reason, history].filter(Boolean).join('\n');
  let schedule = '待团队确认：人力、依赖、范围、节假日与剩余工作量。原日期/人日仅保留讨论。';
  if (old[8] === 'S0' && change.status) schedule = '需重估：原 S0 只排 3 人日验证，已发现实现缺口；开发修复不能隐含在验证工时中。';
  if (old[3] === '已完成') schedule = '待排复验：原估 0 人日不是当前复验工时；取得证据后确定是否开发及安排日期。';
  row.push(nature, evidenceKind, '未完成端到端验收', '未确认');
  rows.push(row);
  functionRefs.set(id, refs);
  const excerpt = refs.map(ref => {
    const req = reqs.get(ref);
    const filter=excerptFilters[id]?.[ref];
    const selected=filter ? req.text.split('\n').filter(line=>filter.test(line)) : req.text.split('\n');
    if(!selected.length) throw new Error('Empty excerpt '+id+' '+ref);
    return ref + ' [' + req.level + '] ' + selected.join(' ');
  }).join('\n');
  evidenceRows.push([id, nature, refs.join('；'), excerpt, evidenceText, proves, limit, row[14] + '\n留证：环境/版本、前置配置、输入、实际与预期、截图或脱敏接口响应、测试人及时间。', old[3], reason + '\n' + history, schedule]);
  expected[id] = { row: i + 2, status: row[3], nature, refs, evidenceKind, name: row[2], previous: old[3] };
  if (change.reason || row[3] !== old[3]) changes.push({ id, previous: old[3], status: row[3], reason });
}

const colors = { navy: '#17324B', teal: '#216D89', blue: '#E8F2F7', amber: '#FFF1CC', red: '#FCE8E6', border: '#D8E1E8', ink: '#263747' };
function set(sheet, address, value) { sheet.getRange(address).values = [[value]]; }
function formula(sheet, address, value) { sheet.getRange(address).formulas = [[value]]; }
function applyHeader(sheet, range) {
  const r = sheet.getRange(range);
  r.format.fill = colors.teal;
  r.format.font = {name: 'Arial', size: 10, color: '#FFFFFF', bold: true};
  r.format.wrapText = true;
  r.format.horizontalAlignment = 'center';
  r.format.verticalAlignment = 'center';
  r.format.rowHeight = 30;
}
function estimateLines(value, width) {
  return String(value ?? '').split('\n').reduce((sum, line) => {
    const units = Array.from(line).reduce((n, ch) => n + (/[^\x00-\x7F]/.test(ch) ? 2 : 1), 0);
    return sum + Math.max(1, Math.ceil(units / Math.max(8, width - 3)));
  }, 0);
}
function fitRows(sheet, values, start, widths, max = 409) {
  values.forEach((row, i) => {
    const lines = Math.max(...row.map((v, j) => estimateLines(v, widths[j] || 22)));
    const needed = Math.max(46, lines * 14 + 10);
    if (needed > max) console.log('Tall row check', sheet.name, start + i, needed);
    sheet.getRange(`A${start+i}:${col(widths.length)}${start+i}`).format.rowHeight = Math.min(max, needed);
  });
}
function col(n) { let x=''; while(n>0) { n--; x=String.fromCharCode(65+n%26)+x; n=Math.floor(n/26); } return x; }
function widthsFor(sheet, widths, end) {
  widths.forEach((w,i) => { sheet.getRange(`${col(i+1)}1:${col(i+1)}${end}`).format.columnWidth = w; });
}

const main = wb.worksheets.getItem('功能完成排期');
main.getRange('A2:W91').values = rows;
main.getRange('A1:W1').values = [[...data.sheets['功能完成排期'].rows[0].map((v,i) => ({3:'实现判断（非验收）',4:'现状记录及本次核查',8:'原拟迭代/历史安排',9:'原拟开始（未确认）',10:'原拟完成（未确认）',11:'原粗估人日（未确认）',14:'建议验收场景',15:'原文编号/设计章节',18:'修订及跟进说明'}[i] || v)), '需求性质', '证据类型', '端到端验收', '排期确认']];
main.getRange('A1:W91').format.font = {name:'Arial',size:10,color:colors.ink};
main.getRange('A1:W91').format.wrapText = true;
main.getRange('A1:W91').format.verticalAlignment = 'top';
const mainTable=main.tables.add('A1:W91',true,'FunctionSchedule');mainTable.style='TableStyleMedium2';mainTable.showFilterButton=true;
applyHeader(main,'A1:W1');
const mainWidths = [13,21,34,17,56,68,12,24,23,22,22,18,29,42,70,34,18,22,68,29,26,25,18];
widthsFor(main,mainWidths,91);
fitRows(main,rows,2,mainWidths);
main.getRange('J2:K91').setNumberFormat('yyyy-mm-dd');
main.getRange('L2:L91').setNumberFormat('0');
main.getRange('R2:R91').setNumberFormat('yyyy-mm-dd');
main.getRange('D2:D91').dataValidation = {rule:{type:'list',values:statusSet}};
main.getRange('W2:W91').dataValidation = {rule:{type:'list',values:['未确认','已确认']}};
main.getRange('V2:V91').dataValidation = {rule:{type:'list',values:['未完成端到端验收','验收通过','验收失败']}};
main.getRange('D2:D91').format.fill = '#FFFFFF';
for (const [status, fill] of [['部分完成',colors.amber],['待完成',colors.red],['待复验',colors.blue],['待验证',colors.blue]]) main.getRange('D2:D91').conditionalFormats.add('containsText',{text:status,format:{fill}});
main.getRange('W2:W91').format.fill = colors.amber;
main.freezePanes.unfreeze();
main.freezePanes.freezeRows(1);
main.freezePanes.freezeColumns(4);

const overview = wb.worksheets.getItem('排期总览');
set(overview,'A1','Onward 业务功能对照与实施排期（修订讨论稿）');
set(overview,'A2','需求基线：v0.1.0；原表：2026-09-07；核查：2026-09-08。90 项为功能拆分，非 90 条独立原文需求。');
set(overview,'A4','功能拆分项');
set(overview,'C4','验收通过项');
set(overview,'E4','部分完成判断');
set(overview,'G4','待验证/复验');
formula(overview,'A5',"=COUNTA('功能完成排期'!A2:A91)");
formula(overview,'C5',"=COUNTIFS('功能完成排期'!V2:V91,\"验收通过\")");
formula(overview,'E5',"=COUNTIFS('功能完成排期'!D2:D91,\"部分完成\")");
formula(overview,'G5',"=COUNTIFS('功能完成排期'!D2:D91,\"待验证\")+COUNTIFS('功能完成排期'!D2:D91,\"待复验\")");
set(overview,'A6','待完成判断');
set(overview,'C6','原拟双周迭代数');
set(overview,'E6','原拟完成（未确认）');
set(overview,'G6','原粗估人日');
formula(overview,'A7',"=COUNTIFS('功能完成排期'!D2:D91,\"待完成\")");
overview.getRange('C7').clear({applyTo:'contents'});
set(overview,'C7',21);
overview.getRange('C7').setNumberFormat('0');
formula(overview,'G7',"=SUM('功能完成排期'!L2:L91)");
set(overview,'A8','修订口径与排期待确认事项');
set(overview,'A9','范围：保留原 90 项及历史估算。新增原文覆盖记录；GLPI、Chatwoot、Keycloak、项目独立部署等架构要求尚未按原文验收。');
set(overview,'A10','证据：原文映射已复核；部分功能完成静态核查或入口定位，未重跑 90 项运行验收。测试源码不等于执行通过，原 12 项“已完成”现列待复验。');
set(overview,'A11','排期：21 个双周迭代和 889 人日来自原表，尚未获团队确认。已发现缺口不能只按 3 人日验证关闭；修复、复验与架构工作需补估。');
set(overview,'A12','团队：原表假设产品/项目负责人 1、后端 2、前端 2、QA 1，设计/运维按需。原文第 15 章另建议架构 1、后端 2、SRE 1、QA 1、前端/报表 1，产品/安全兼职，路线图为 20 周。');
set(overview,'A13','冻结前确认：采用原架构还是批准替代、项目范围、真实人员、任务拆解/依赖、节假日、外部资源及验收负责人。原日期保留供讨论，不是交付承诺。');
set(overview,'A15','原拟里程碑（全部待团队确认）');
set(overview,'B16','原拟日期');
set(overview,'C16','拟交付结果（未验收）');
overview.getRange('A9:H13').format.wrapText = true;
overview.getRange('A9:H13').format.rowHeight = 46;
overview.getRange('A1:H2').format.wrapText = true;
overview.getRange('A1:H1').format.rowHeight = 36;
overview.getRange('A2:H2').format.rowHeight = 30;
overview.getRange('A4:H7').format.wrapText = true;
overview.getRange('A4:H7').format.rowHeight = 30;
overview.getRange('C7').format.font = {name:'Arial',size:16,bold:true,color:colors.ink};
overview.getRange('E7').format.font = {name:'Arial',size:12,color:colors.ink};
overview.getRange('E7').setNumberFormat('yyyy-mm-dd');
for (let r=17;r<=22;r++) overview.getRange(`A${r}:H${r}`).format.rowHeight = 24;

const priorities = wb.worksheets.getItem('优先执行清单');
set(priorities,'E1','实现判断（非验收）');
set(priorities,'G1','原拟迭代');
set(priorities,'H1','原拟日期（未确认）');
set(priorities,'I1','修订后下一步');
set(priorities,'J1','建议验收场景');
const priorityOriginal = data.sheets['优先执行清单'].rows.slice(1);
for(let i=0;i<priorityOriginal.length;i++) {
  const rr=i+2, id=priorityOriginal[i][1], src=expected[id].row;
  for(const [dst,srcCol] of [['C','B'],['D','C'],['E','D'],['F','G'],['G','I'],['H','K'],['I','F'],['J','O']]) formula(priorities,`${dst}${rr}`,`='功能完成排期'!${srcCol}${src}`);
}
priorities.getRange('A1:J79').format.wrapText = true;
const priorityWidths=[9,14,22,34,19,12,18,23,76,76];
widthsFor(priorities,priorityWidths,79);
fitRows(priorities,priorityOriginal.map(r=>{const m=rows[expected[r[1]].row-2];return [r[0],r[1],m[1],m[2],m[3],m[6],m[8],m[10],m[5],m[14]];}),2,priorityWidths);
priorities.getRange('H2:H79').setNumberFormat('yyyy-mm-dd');
applyHeader(priorities,'A1:J1');
priorities.freezePanes.unfreeze(); priorities.freezePanes.freezeRows(1); priorities.freezePanes.freezeColumns(2);

const iteration = wb.worksheets.getItem('迭代计划');
set(iteration,'C1','原拟开始（未确认）');set(iteration,'D1','原拟结束（未确认）');set(iteration,'E1','原拟功能范围（修订名称）');set(iteration,'G1','原估人日（待重估）');set(iteration,'H1','修订后门槛及排期风险');
for(let i=0;i<21;i++) {
  const rr=i+2, iter='S'+i, assigned=rows.filter(r=>r[8]===iter);
  set(iteration,`E${rr}`,assigned.length ? assigned.map(r=>r[0]+' '+r[2]).join('\n') : '全链路回归与 UAT（未单独估算）');
  formula(iteration,`F${rr}`,`=COUNTIFS('功能完成排期'!I2:I91,A${rr})`);
  formula(iteration,`G${rr}`,`=SUMIFS('功能完成排期'!L2:L91,'功能完成排期'!I2:I91,A${rr})`);
  const text=i===0 ? '原 9 项先复核并登记缺口，不能承诺均在 S0 完成。电话、专家、登录、SLA、合并和 Bot 治理需要补实现/重估；原 12 项待复验另行安排。' : i===20 ? '原表本轮记 0 人日，仅因没有功能行归属，不代表回归免费。需补测试/修复容量，取得逐项证据、关闭阻塞缺陷并由业务负责人签收。' : '原拟日期尚未确认。逐项满足修订后的验收场景并留证；先确认前置依赖、外部资源、团队容量和架构差异，失败修复须另估。';
  set(iteration,`H${rr}`,text);
}
const iterWidths=[11,55,23,23,73,13,24,82,39];
widthsFor(iteration,iterWidths,22);
iteration.getRange('A1:I22').format.wrapText=true;
applyHeader(iteration,'A1:I1');
fitRows(iteration,data.sheets['迭代计划'].rows.slice(1).map((r,i)=>{const x=r.slice(); x[4]=rows.filter(v=>v[8]==='S'+i).map(v=>v[0]+' '+v[2]).join('\n');x[7]=iteration.getRange(`H${i+2}`).values[0][0];return x;}),2,iterWidths);
iteration.getRange('C2:D22').setNumberFormat('yyyy-mm-dd');
iteration.freezePanes.unfreeze(); iteration.freezePanes.freezeRows(1);iteration.freezePanes.freezeColumns(2);

const external = wb.worksheets.getItem('外部准备项');
set(external,'C1','原拟最晚时间（未确认）');
set(external,'G1','准备状态（原表）');
set(external,'H1','依赖校正与确认事项');
const externalNotes={
  2:'原时间等于 S0 结束，但 FUN-004 的身份源/测试账号需在 S0 验证前提供。改期与账号可用性待 IT 确认。',
  3:'需确认当前实际邮箱配置，不沿用“SMTP 未启用”。收件自动建单与发件通知分开准备及验收。',
  4:'原时间晚于 S0 的 SLA 验证。S0 前至少准备批准的测试目标、时区/日历、暂停与升级规则。',
  5:'原时间晚于 S0 的 Bot 发布验证。S0 前需测试用获批知识、敏感意图、阈值和审批账号。',
  6:'原时间晚于 S0 的专家交接验证。S0 前需测试专家、外部引用、OLA 与交接/回交模板。',
  7:'先确认采用 Chatwoot Community 与官方 Cloud API；号码/模板、商业责任和 Readiness 批准后才能启用。',
  8:'日期待确认；阿语须人工审校并检查 RTL、表格、通知及导出，不仅检查翻译字符串。',
  9:'先批准监控目标与访问范围，禁止制造交易数据风险；真实样本和维护窗口需可复现。',
  10:'QA 规则/评分卡需版本化；排除薪资、奖金、HR 绩效、考勤排班。',
  11:'需报告来源、截止水位、模板/查询版本、对账和审批/交付证据，不只提供版式。',
  12:'上传验证不应等待完整退出政策；S0 前先准备文件类型/大小/压缩规则与安全样本，留存与退出范围另确认。',
};
for (const [r,text] of Object.entries(externalNotes)) set(external,`H${r}`,text);
const extWidths=[27,66,26,42,45,27,24,83];
widthsFor(external,extWidths,12);external.getRange('A1:H12').format.wrapText=true;applyHeader(external,'A1:H1');
fitRows(external,data.sheets['外部准备项'].rows.slice(1).map((r,i)=>{const x=r.slice();x[7]=externalNotes[i+2];return x;}),2,extWidths);
external.getRange('C2:C12').setNumberFormat('yyyy-mm-dd');

function addEvidenceSheet(name, subtitle, headers, values, widths, tableName) {
  const sheet=wb.worksheets.add(name), last=values.length+4, finalCol=col(headers.length);
  set(sheet,'A1',name); sheet.mergeCells(`A1:${finalCol}1`);
  set(sheet,'A2',subtitle); sheet.mergeCells(`A2:${finalCol}2`);
  sheet.getRange(`A4:${finalCol}4`).values=[headers];
  sheet.getRange(`A5:${finalCol}${last}`).values=values;
  sheet.getRange(`A1:${finalCol}${last}`).format.font={name:'Arial',size:10,color:colors.ink};
  sheet.getRange(`A1:${finalCol}${last}`).format.wrapText=true;
  sheet.getRange(`A1:${finalCol}${last}`).format.verticalAlignment='top';
  widthsFor(sheet,widths,last);
  const table=sheet.tables.add(`A4:${finalCol}${last}`,true,tableName); table.style='TableStyleMedium2';table.showFilterButton=true;
  applyHeader(sheet,`A4:${finalCol}4`);
  sheet.getRange(`A1:${finalCol}1`).format.font={name:'Arial',size:15,bold:true,color:colors.navy};
  sheet.getRange(`A1:${finalCol}1`).format.rowHeight=30;
  sheet.getRange(`A2:${finalCol}2`).format.rowHeight=38;
  fitRows(sheet,values,5,widths);
  sheet.freezePanes.freezeRows(4);sheet.freezePanes.freezeColumns(1);sheet.showGridLines=false;
  return sheet;
}
const evidence = addEvidenceSheet('逐项核查依据','核查日期：2026-09-08。代码依据为当前本地工作树（含未提交改动），HEAD 84d739ac08a9；本次未执行单元测试或端到端验收。原表状态保留在 I 列。',
  ['功能ID','需求性质','真实编号/章节','原文摘录（非改写）','代码/历史证据定位','证据能证明什么','不能证明什么/剩余风险','建议验证与留证','原表状态','修订理由','排期待确认事项'],evidenceRows,[14,33,37,120,100,70,84,90,16,88,78],'FunctionEvidence');

const approvedPrefixes=/^(FND|IAM|TKT|DCH|REG|EVT|SLA|MON|KB|QA|RPT|FLT|DAT|SEC|AVL|PERF|BKP|BR|C)-/;
const rawReqs=[...reqs.values()].filter(r=>approvedPrefixes.test(r.id)||r.id.startsWith('§'));
const scopeRows=rawReqs.map(req=>{
  const linked=[...functionRefs].filter(([,refs])=>refs.includes(req.id)).map(([id])=>id);
  let coverage=linked.length?'相关功能已映射；不代表整条需求已实现':'原 90 项未单列；不能从业务完成比例推定满足';
  let action=linked.length?'按原文全文核对各关联拆分项；实施方案及跨项验收仍需确认。':'补入范围评审，明确独立工作项、负责人、估算与证据；本次不擅自排期。';
  if (/^(FND|EVT|FLT|AVL|PERF|BKP)-/.test(req.id) || ['C-03','C-05','C-06','C-09','MON-001','SEC-001','SEC-002','SEC-005','SEC-006','SEC-007','SEC-008'].includes(req.id)) action='架构/工程工作单独核查。当前自研系统不能凭相似界面视为符合 GLPI/Ops Core 和每项目独立部署基线；需遵循原设计或取得变更批准。';
  if (['QA-005','RPT-005','DCH-009','DAT-005'].includes(req.id)) action='保留原文边界/工程约束，加入相应模块验收；未单列不表示可忽略。';
  if(req.id==='FND-001') coverage='已移除 FUN-001 错误映射；企业管理员登录不能证明项目自动部署';
  return [req.id,req.level,req.section,req.text,linked.join('\n')||'未单列',coverage,action,'未完成验收'];
});
const sourceSheet=addEvidenceSheet('需求原文与范围','来源：D:/桌面/Onward-HelpDesk-Platform-内部开发需求与总体设计-v0.1.0.docx。编号及摘录直接读取原文；§ 表示真实章节，不是新增需求 ID。',
 ['原文编号/章节','原文级别','原文位置','原文内容','关联功能ID','覆盖范围说明','后续处理','验收结论'],scopeRows,[19,34,56,125,30,73,100,24],'RequirementScope');

// Keep discussion inputs next to the existing overview instead of inventing new dates.
const decisions=[
 ['待确认事项','原文/原表依据','当前结论','确认负责人','确认日期'],
 ['总体架构','原文第 1.2、1.3、5、6、12 章','待决定：遵循原文组件和事实源，或先批准替代方案','待指定',null],
 ['真实团队与可用工时','原文第 15.1 节与原表团队配置不同','待确认每人投入、职责和并行容量','待指定',null],
 ['剩余估算','原表 889 人日；S0 9 项各 3 人日','需补已知缺口修复、12 项复验、未单列架构与最终回归估算','待指定',null],
 ['前置依赖','外部准备项原日期晚于部分 S0 验证','先确认 S0 测试身份源、SLA、Bot、专家和上传规则可用时间','待指定',null],
 ['计划批准','原文 20 周路线图；原表 21 个双周迭代','日期均未确认，完成拆解/容量和依赖检查后由团队批准','待指定',null],
 ['原表与原文定位','原表：Onward_业务功能对照与实施排期_最终版_20260907.xlsx','原 5 张表和 90 编号保留；新增两表记录逐项证据及原文范围','不适用',null],
 ['验证记录边界','静态核查、代码入口和历史描述分开','本次未运行 90 项验收；优先清单保留原 78 项，新增待复验 12 项在主表列示','待指定',null],
];
overview.getRange('A25:E32').values=decisions;
overview.getRange('A25:E32').format.wrapText=true;
overview.getRange('A25:E32').format.font={name:'Arial',size:10,color:colors.ink};
overview.getRange('A25:E32').format.verticalAlignment='top';
applyHeader(overview,'A25:E25');
fitRows(overview,decisions.slice(1),26,[27,22,22,22,22]);
overview.getRange('D26:E32').format.fill=colors.amber;
overview.getRange('E26:E32').setNumberFormat('yyyy-mm-dd');

for(const [sheet,range,name] of [[priorities,'A1:J79','PriorityActions'],[iteration,'A1:I22','IterationPlan'],[external,'A1:H12','ExternalInputs']]) {
  const table=sheet.tables.add(range,true,name);table.style='TableStyleMedium2';table.showFilterButton=true;
}
set(main,'V2','验收通过');
wb.recalculate();
if(overview.getRange('C5').values[0][0]!==1) throw new Error('Acceptance summary failed to react');
set(main,'V2','未完成端到端验收');
wb.recalculate();
const actualOverview=['A5','C5','E5','G5','A7','C7','G7'].map(a=>[a,overview.getRange(a).values[0][0]]);
console.log('Overview:',JSON.stringify(actualOverview));
const counts=Object.fromEntries(statusSet.map(s=>[s,rows.filter(r=>r[3]===s).length]));
if(overview.getRange('A5').values[0][0]!==90 || overview.getRange('C5').values[0][0]!==0 || overview.getRange('G7').values[0][0]!==889) throw new Error('Overview reconciliation failed');
const bad=await wb.inspect({kind:'match',searchTerm:'#REF!|#DIV/0!|#VALUE!|#NAME\\?|#NUM!|#SPILL!',options:{useRegex:true,maxResults:20},maxChars:3000});
console.log('Formula scan:',bad.ndjson);
const filename='Onward_业务功能对照与实施排期_修订讨论稿_20260908.xlsx';
const outputPath=path.join(out,filename);
const file=await SpreadsheetFile.exportXlsx(wb); await file.save(outputPath);
await fs.writeFile(path.join(out,'support/verification.json'),JSON.stringify({counts,sourceCount:scopeRows.length,expected,changes,overview:actualOverview,output:outputPath},null,2));
wb = await SpreadsheetFile.importXlsx(await FileBlob.load(outputPath));
await render('排期总览','A1:H22','after-overview');
await render('排期总览','A25:E32','after-decisions');
await render('功能完成排期','A1:F4','after-detail');
await render('功能完成排期','O1:W3','after-detail-evidence');
await render('迭代计划','A1:D4','after-iterations');
await render('外部准备项','F1:H4','after-external');
await render('优先执行清单','G1:J3','after-priority');
await render('逐项核查依据','E4:G6','after-evidence');
await render('需求原文与范围','A4:D6','after-source');
console.log('Exported',outputPath,'counts',JSON.stringify(counts),'sources',scopeRows.length);
