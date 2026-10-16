package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleDashboard serves a zero-dependency interactive Web UI to test bank statement uploads live in browser.
func HandleDashboard(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Azaman — Shazam for Bank Statements</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #090d16;
      --card: #111827;
      --border: #1f2937;
      --text: #f3f4f6;
      --muted: #9ca3af;
      --accent: #10b981;
      --accent-glow: rgba(16, 185, 129, 0.2);
      --red: #ef4444;
      --amber: #f59e0b;
      --blue: #3b82f6;
      --purple: #8b5cf6;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--text);
      font-family: 'Plus Jakarta Sans', sans-serif;
      min-height: 100vh;
      padding: 24px;
      line-height: 1.5;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding-bottom: 24px;
      border-bottom: 1px solid var(--border);
      margin-bottom: 28px;
      flex-wrap: wrap;
      gap: 16px;
    }
    .logo {
      font-size: 24px;
      font-weight: 800;
      letter-spacing: -0.5px;
      display: flex;
      align-items: center;
      gap: 10px;
    }
    .logo span { color: var(--accent); }
    .badges { display: flex; gap: 8px; flex-wrap: wrap; }
    .badge {
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px;
      font-weight: 600;
      padding: 4px 10px;
      border-radius: 999px;
      border: 1px solid var(--border);
      background: rgba(31, 41, 55, 0.5);
    }
    .badge.green { border-color: rgba(16, 185, 129, 0.3); color: var(--accent); background: var(--accent-glow); }
    .badge.purple { border-color: rgba(139, 92, 246, 0.3); color: var(--purple); background: rgba(139, 92, 246, 0.15); }
    .links a {
      color: var(--muted);
      text-decoration: none;
      font-size: 13px;
      font-weight: 600;
      margin-left: 12px;
      transition: color 0.2s;
    }
    .links a:hover { color: var(--accent); }
    .upload-card {
      background: var(--card);
      border: 2px dashed var(--border);
      border-radius: 16px;
      padding: 36px 24px;
      text-align: center;
      cursor: pointer;
      transition: all 0.2s ease;
      position: relative;
    }
    .upload-card:hover, .upload-card.dragover {
      border-color: var(--accent);
      background: rgba(17, 24, 39, 0.8);
      box-shadow: 0 0 30px var(--accent-glow);
    }
    .upload-card input[type="file"] {
      position: absolute;
      top: 0; left: 0; width: 100%; height: 100%;
      opacity: 0; cursor: pointer;
    }
    .upload-icon { font-size: 40px; margin-bottom: 12px; }
    .upload-title { font-size: 18px; font-weight: 700; margin-bottom: 6px; }
    .upload-desc { color: var(--muted); font-size: 13px; }
    .form-row {
      position: relative;
      z-index: 2; /* sit above the full-card invisible file input so the button and password field are clickable */
      display: flex;
      gap: 12px;
      margin-top: 16px;
      align-items: center;
      justify-content: center;
      flex-wrap: wrap;
    }
    .input-field {
      background: #1f2937;
      border: 1px solid #374151;
      color: var(--text);
      padding: 10px 14px;
      border-radius: 8px;
      font-size: 13px;
      outline: none;
      min-width: 220px;
    }
    .btn {
      background: var(--accent);
      color: #000;
      border: none;
      padding: 10px 22px;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 700;
      cursor: pointer;
      transition: all 0.2s;
    }
    .btn:hover { opacity: 0.9; transform: translateY(-1px); }
    .btn:disabled { opacity: 0.5; cursor: not-allowed; }
    .kpis {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px;
      margin-top: 28px;
    }
    .internal-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 14px 18px;
      margin-bottom: 20px;
    }
    .internal-card .kpi-sub { font-size: 14px; color: var(--text); margin-top: 6px; }
    .kpi-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px;
    }
    .kpi-label { font-size: 12px; font-weight: 600; text-transform: uppercase; color: var(--muted); margin-bottom: 6px; }
    .kpi-value { font-size: 24px; font-weight: 800; font-family: 'JetBrains Mono', monospace; }
    .kpi-value.green { color: var(--accent); }
    .kpi-value.red { color: var(--red); }
    .kpi-sub { font-size: 12px; color: var(--muted); margin-top: 4px; }
    .leaks-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;
      margin-top: 20px;
    }
    .leak-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 18px;
      position: relative;
      overflow: hidden;
    }
    .leak-card::before {
      content: '';
      position: absolute;
      top: 0; left: 0; width: 4px; height: 100%;
    }
    .leak-card.midnight::before { background: var(--purple); }
    .leak-card.urgent2k::before { background: var(--amber); }
    .leak-card.duplicate::before { background: var(--red); }
    .leak-card.bank::before { background: var(--blue); }
    .leak-title { font-size: 14px; font-weight: 700; display: flex; align-items: center; justify-content: space-between; }
    .leak-amount { font-size: 20px; font-weight: 800; font-family: 'JetBrains Mono', monospace; margin: 8px 0; }
    .table-container {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      margin-top: 28px;
      overflow-x: auto;
    }
    table { width: 100%; border-collapse: collapse; font-size: 13px; text-align: left; }
    th {
      background: #1f2937;
      padding: 12px 16px;
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      color: var(--muted);
      border-bottom: 1px solid var(--border);
    }
    td { padding: 12px 16px; border-bottom: 1px solid var(--border); }
    tr:last-child td { border-bottom: none; }
    .tag {
      font-size: 11px;
      font-weight: 600;
      padding: 2px 8px;
      border-radius: 6px;
      display: inline-block;
    }
    .tag.leak { background: rgba(239, 68, 68, 0.15); color: var(--red); }
    .tag.urgent { background: rgba(245, 158, 11, 0.15); color: var(--amber); }
    .tag.tax { background: rgba(59, 130, 246, 0.15); color: var(--blue); }
    .tag.ai { background: rgba(139, 92, 246, 0.2); color: var(--purple); }
    .confidence { font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--accent); }
    .memo-sandbox {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 20px;
      margin-top: 28px;
    }
    .hidden { display: none; }
    #spinner { display: none; margin: 20px auto; width: 40px; height: 40px; border: 4px solid #374151; border-top-color: var(--accent); border-radius: 50%; animation: spin 0.8s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="logo">⚡ AZAMAN <span>SHAZAM</span></div>
      <div class="badges">
        <div class="badge green">🛡️ Zero-Disk RAM (32MB)</div>
        <div class="badge purple" id="laya-badge">⚡ Laya AI Ready</div>
        <div class="badge">OPay · PalmPay · GTB · Kuda · Zenith · +10 banks</div>
      </div>
      <div class="links">
        <a href="/swagger" target="_blank">📖 Swagger Docs</a>
        <a href="/health" target="_blank">🩺 Health</a>
      </div>
    </header>

    <div class="upload-card" id="dropzone">
      <input type="file" id="fileInput" accept=".pdf,.csv,.tsv,.xlsx" />
      <div class="upload-icon">📄</div>
      <div class="upload-title">Drop your Nigerian bank statement here</div>
      <div class="upload-desc">Supports PDF (Password-locked or Plain), CSV, XLSX (OPay, GTBank, Kuda, Moniepoint, Geegpay USD)</div>
      <div class="form-row" onclick="event.stopPropagation()">
        <input type="password" id="pdfPassword" class="input-field" placeholder="PDF Password (if locked, e.g. phone/acct)" />
        <button class="btn" id="uploadBtn" onclick="uploadStatement()">Shazam Statement ⚡</button>
      </div>
    </div>
    <div id="spinner"></div>

    <div id="results" class="hidden">
      <div class="internal-card hidden" id="check-card" style="border-color: var(--red);">
        <div class="kpi-sub" id="check-summary"></div>
      </div>
      <div class="kpis">
        <div class="kpi-card">
          <div class="kpi-label">Total Inflow</div>
          <div class="kpi-value green" id="inflow">₦0.00</div>
          <div class="kpi-sub" id="inflow-sub">Real income (excludes own transfers & savings)</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-label">Total Outflow</div>
          <div class="kpi-value red" id="outflow">₦0.00</div>
          <div class="kpi-sub" id="outflow-sub">Real spending (excludes own transfers & savings)</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-label">Net Retained</div>
          <div class="kpi-value" id="net">₦0.00</div>
          <div class="kpi-sub" id="margin">Margin: 0%</div>
        </div>
        <div class="kpi-card">
          <div class="kpi-label">Estimated Runway</div>
          <div class="kpi-value" id="runway" style="color: var(--blue);">0 mos</div>
          <div class="kpi-sub" id="burn-sub">Avg Monthly Burn: ₦0</div>
        </div>
      </div>

      <div class="internal-card" id="internal-card">
        <div class="leak-title">🔁 Moved between your accounts <span>Not income · Not spending</span></div>
        <div class="kpi-sub" id="internal-summary">—</div>
      </div>

      <div class="internal-card hidden" id="uncat-card">
        <div class="leak-title">❓ Uncategorized <span>We'd rather not guess</span></div>
        <div class="kpi-sub" id="uncat-summary"></div>
      </div>

      <div class="leaks-grid">
        <div class="leak-card midnight">
          <div class="leak-title">🌙 Midnight Cash Leak <span>10PM - 4AM</span></div>
          <div class="leak-amount" id="midnight-amt" style="color: var(--purple);">₦0.00</div>
          <div class="kpi-sub" id="midnight-sub">0 transactions (Chowdeck, Lounges, late rides)</div>
        </div>
        <div class="leak-card urgent2k">
          <div class="leak-title">💸 Urgent 2k Favors <span>P2P Drain</span></div>
          <div class="leak-amount" id="urgent-amt" style="color: var(--amber);">₦0.00</div>
          <div class="kpi-sub" id="urgent-sub">0 transfers (avg: ₦0)</div>
        </div>
        <div class="leak-card duplicate">
          <div class="leak-title">⚠️ Duplicate Debits <span>Potential Refund</span></div>
          <div class="leak-amount" id="dup-amt" style="color: var(--red);">₦0.00</div>
          <div class="kpi-sub" id="dup-sub">0 suspect duplicate pairs</div>
        </div>
        <div class="leak-card bank">
          <div class="leak-title">🏦 Bank Surcharges <span>Levies & Fees</span></div>
          <div class="leak-amount" id="fees-amt" style="color: var(--blue);">₦0.00</div>
          <div class="kpi-sub" id="fees-sub">EMTL, USSD, Alerts, Stamp Duty</div>
        </div>
      </div>

      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Date</th>
              <th>Clean Merchant</th>
              <th>Category</th>
              <th>Classification</th>
              <th>Amount</th>
              <th>Confidence</th>
              <th>Flags</th>
            </tr>
          </thead>
          <tbody id="tx-table-body"></tbody>
        </table>
      </div>
    </div>

    <div class="memo-sandbox">
      <div class="leak-title" style="margin-bottom: 12px;">⚡ Single Memo Fast Decoder (Sub-5ms)</div>
      <div class="form-row" style="justify-content: flex-start;">
        <input type="text" id="memoInput" class="input-field" style="flex: 1;" placeholder="Paste raw Nigerian memo e.g. TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00" value="TRF 093201/CHOWDECK_LAGOS_NG/UBAS_00" />
        <button class="btn" onclick="decodeSingleMemo()">Decode Memo</button>
      </div>
      <div id="memoResult" style="margin-top: 14px; font-family: 'JetBrains Mono', monospace; font-size: 13px; color: var(--accent);"></div>
    </div>
  </div>

  <script>
    const fmtNaira = n => '₦' + Number(n).toLocaleString('en-NG', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

    async function checkHealth() {
      try {
        const res = await fetch('/health');
        const data = await res.json();
        const badge = document.getElementById('laya-badge');
        if (data.ai_decision_engine && data.ai_decision_engine.status === 'connected') {
          badge.textContent = '⚡ Laya AI: Connected';
          badge.className = 'badge purple';
        } else {
          badge.textContent = '⚡ Laya AI: Standby';
          badge.className = 'badge';
        }
      } catch (e) {}
    }
    checkHealth();

    document.getElementById('fileInput').addEventListener('change', e => {
      if (e.target.files.length > 0) {
        document.querySelector('.upload-title').textContent = 'Selected: ' + e.target.files[0].name;
      }
    });

    const esc = v => String(v).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

    async function uploadStatement() {
      const file = document.getElementById('fileInput').files[0];
      if (!file) {
        alert('Please select a bank statement file (PDF, CSV, or XLSX)');
        return;
      }

      const formData = new FormData();
      formData.append('statement', file);
      const pwd = document.getElementById('pdfPassword').value;
      if (pwd) formData.append('password', pwd);

      document.getElementById('spinner').style.display = 'block';
      document.getElementById('results').classList.add('hidden');
      document.getElementById('uploadBtn').disabled = true;

      try {
        const start = performance.now();
        const res = await fetch('/api/v1/shazam/audit', { method: 'POST', body: formData });
        const latency = (performance.now() - start).toFixed(0);

        if (!res.ok) {
          const err = await res.json();
          alert('Audit Failed: ' + (err.error || 'Unknown error'));
          return;
        }

        const report = await res.json();
        renderAudit(report, latency);
      } catch (err) {
        alert('Failed to connect to Azaman Engine: ' + err.message);
      } finally {
        document.getElementById('spinner').style.display = 'none';
        document.getElementById('uploadBtn').disabled = false;
      }
    }

    function renderAudit(report, clientLatency) {
      document.getElementById('results').classList.remove('hidden');

      // Statement check: warn when parsed totals disagree with the totals printed on the statement
      const chk = report.statement_check || {};
      document.getElementById('check-card').classList.toggle('hidden', !chk.checked || chk.ok);
      document.getElementById('check-summary').textContent = chk.checked && !chk.ok
        ? '⚠️ This statement may not have been read fully: read ' + chk.parsed_rows + ' of ' + chk.stated_rows +
          ' rows, debits ' + fmtNaira(chk.parsed_debit) + ' vs ' + fmtNaira(chk.stated_debit) +
          ', credits ' + fmtNaira(chk.parsed_credit) + ' vs ' + fmtNaira(chk.stated_credit) + '. Treat the numbers below with care.'
        : '';

      const s = report.summary;
      document.getElementById('inflow').textContent = fmtNaira(s.total_inflow);
      document.getElementById('outflow').textContent = fmtNaira(s.total_outflow);
      document.getElementById('net').textContent = fmtNaira(s.net_retained);
      document.getElementById('net').className = 'kpi-value ' + (s.net_retained >= 0 ? 'green' : 'red');
      document.getElementById('margin').textContent = 'Margin: ' + s.profit_margin_pct + '%';
      document.getElementById('runway').textContent = s.runway_months >= 900 ? '∞ (Self-Sustaining)' : s.runway_months + ' mos';
      document.getElementById('burn-sub').textContent = 'Monthly Burn: ' + fmtNaira(s.avg_monthly_burn);

      // Uncategorized: spending the engine couldn't place with confidence
      const unc = report.uncategorized || { count: 0, total: 0 };
      document.getElementById('uncat-summary').textContent = fmtNaira(unc.total) + ' across ' + unc.count +
        ' payment(s) we couldn\'t confidently place. Counted in your spending, not in any leak.';
      document.getElementById('uncat-card').classList.toggle('hidden', !unc.count);

      // Own-account and savings movements, shown separately so the totals above stay "real"
      const it = report.internal_transfers || {};
      const moneyShort = n => '₦' + Math.round(Number(n) || 0).toLocaleString('en-NG');
      const inTotal = (it.savings_withdrawals || 0) + (it.self_transfers_in || 0);
      const outTotal = (it.savings_deposits || 0) + (it.self_transfers_out || 0);
      document.getElementById('internal-summary').textContent = (inTotal || outTotal)
        ? moneyShort(inTotal) + ' in (savings ' + moneyShort(it.savings_withdrawals) + ', own transfers ' + moneyShort(it.self_transfers_in) + ')' +
          ' · ' + moneyShort(outTotal) + ' out (savings ' + moneyShort(it.savings_deposits) + ', own transfers ' + moneyShort(it.self_transfers_out) + ')'
        : 'No transfers between your own accounts or savings in this statement.';

      const leaks = report.cash_leaks;
      document.getElementById('midnight-amt').textContent = fmtNaira(leaks.midnight_leak.total_amount);
      document.getElementById('midnight-sub').textContent = leaks.midnight_leak.count + ' late night drains (10PM - 4AM)';

      document.getElementById('urgent-amt').textContent = fmtNaira(leaks.urgent_2k.total_amount);
      document.getElementById('urgent-sub').textContent = leaks.urgent_2k.count + ' favors (avg: ' + fmtNaira(leaks.urgent_2k.average_transfer_size) + ')';

      document.getElementById('dup-amt').textContent = fmtNaira(leaks.duplicate_debits.total_potential_refund);
      document.getElementById('dup-sub').textContent = leaks.duplicate_debits.count + ' potential refund items';

      document.getElementById('fees-amt').textContent = fmtNaira(leaks.bank_charges.total_charges);
      document.getElementById('fees-sub').textContent = leaks.bank_charges.count + ' surcharges (EMTL, USSD, VAT)';

      const tbody = document.getElementById('tx-table-body');
      tbody.innerHTML = '';

      (report.transactions || []).forEach(tx => {
        const tr = document.createElement('tr');
        let flags = '';
        if (tx.flags.is_midnight_leak) flags += '<span class="tag leak">Midnight</span> ';
        if (tx.flags.is_urgent_2k) flags += '<span class="tag urgent">Urgent 2k</span> ';
        if (tx.flags.is_duplicate_debit) flags += '<span class="tag leak">Duplicate</span> ';
        if (tx.flags.is_tax_deductible) flags += '<span class="tag tax">Tax-Deductible</span> ';

        const amtColor = tx.amount > 0 ? 'color: var(--accent);' : 'color: var(--text);';
        const sign = tx.amount > 0 ? '+' : '';

        tr.innerHTML =
          '<td style="font-family: JetBrains Mono, monospace; font-size: 12px; color: var(--muted);">' + esc(tx.date || '') + '</td>' +
          '<td><strong>' + esc(tx.merchant || 'General') + '</strong><div style="font-size: 11px; color: var(--muted);">' + esc(tx.raw_description || '') + '</div></td>' +
          '<td>' + esc(tx.category || '') + '</td>' +
          '<td><span style="font-size: 12px; color: var(--muted);">' + esc(tx.classification || '') + '</span></td>' +
          '<td style="font-family: JetBrains Mono, monospace; font-weight: 700; ' + amtColor + '">' + sign + fmtNaira(tx.amount) + '</td>' +
          '<td><span class="confidence">' + (tx.confidence * 100).toFixed(0) + '%</span></td>' +
          '<td>' + (flags || '<span style="color: var(--muted); font-size: 11px;">—</span>') + '</td>';
        tbody.appendChild(tr);
      });
    }

    async function decodeSingleMemo() {
      const memo = document.getElementById('memoInput').value;
      if (!memo) return;

      const res = await fetch('/api/v1/shazam/decode-memo', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ memo: memo, amount: 5000 })
      });

      const data = await res.json();
      const d = data.decoded;
      document.getElementById('memoResult').innerHTML = 
        '<strong>Clean Merchant:</strong> ' + d.merchant + ' | ' +
        '<strong>Category:</strong> ' + d.category + ' | ' +
        '<strong>Classification:</strong> ' + d.classification + ' | ' +
        '<strong>Confidence:</strong> ' + (d.confidence * 100).toFixed(0) + '%';
    }
  </script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
