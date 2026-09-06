import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';

const source = readFileSync(new URL('./panel.go', import.meta.url), 'utf8');
const script = source.split('<script>')[1].split('</script>')[0];

function panel(pathname = '/v0/resource/plugins/codex-health-monitor/panel') {
  const elements = new Map();
  const context = vm.createContext({
    location: {pathname},
    document: {getElementById(id) {
      if (!elements.has(id)) elements.set(id, {textContent: '', innerHTML: '', disabled: false});
      return elements.get(id);
    }},
  });
  // Evaluate the shipped functions without connecting to a live management API.
  vm.runInContext(script.slice(0, script.indexOf("document.querySelectorAll('input[name=\"scheduleMode\"]')")), context);
  return {elements, run: code => vm.runInContext(code, context)};
}

test('panel API follows the loaded plugin ID', () => {
  for (const id of ['codex-health-monitor', 'codex-health-monitor-linux-arm64', 'codex-health-monitor-v0.1.8']) {
    for (const suffix of ['', '/']) {
      const p = panel(`/v0/resource/plugins/${id}/panel${suffix}`);
      assert.equal(p.run('API'), `/v0/management/plugins/${id}`);
    }
  }
});

test('history pagination handles page changes, refresh and shrinking history', () => {
  const p = panel();
  p.run(`renderHistory([{started_at:'2026-09-06T00:00:00Z',trigger:'manual',accounts:Array.from({length:23},(_,i)=>({email:'user-'+i,status:'healthy',healthy:true}))}])`);
  assert.equal((p.elements.get('history').innerHTML.match(/<tr>/g) || []).length, 10);
  p.run('historyPage=3;renderHistoryPage()');
  assert.equal((p.elements.get('history').innerHTML.match(/<tr>/g) || []).length, 3);
  assert.equal(p.elements.get('historyNext').disabled, true);
  p.run('renderHistory([{accounts:historyRows.map(row=>row.account)}])');
  assert.equal(p.run('historyPage'), 3);
  p.run('renderHistory([{accounts:[{email:"remaining",status:"not_checked"}]}])');
  assert.equal(p.run('historyPage'), 1);
  p.run('renderHistory([])');
  assert.equal(p.elements.get('historyPrev').disabled, true);
  assert.equal(p.elements.get('historyNext').disabled, true);
});

test('failed discovery remains visible in history and escapes its message', () => {
  const p = panel();
  p.run(`renderHistory([{trigger:'scheduled',error_code:'account_discovery_failed',error_message:'Failed <script>bad</script>',accounts:null}])`);
  assert.match(p.elements.get('history').innerHTML, /Failed &lt;script&gt;bad&lt;\/script&gt;/);
  assert.match(p.elements.get('history').innerHTML, /响应异常/);
});
