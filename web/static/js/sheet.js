// Roster and score sheet tools for teachers: live statistics, selecting students,
// applying one change to many students at once (with undo), spreadsheet-style keys.
// Nothing is saved until the teacher presses Save draft or Submit; the server re-checks every score.
(function () {
  'use strict';
  var form = document.querySelector('form[data-sheet]');
  if (!form) return;

  var $ = function (sel, root) { return (root || form).querySelector(sel); };
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || form).querySelectorAll(sel)); };

  var MAX = { ca: parseFloat(form.getAttribute('data-max-ca')) || 40, exam: parseFloat(form.getAttribute('data-max-exam')) || 60 };
  var PASS = parseFloat(form.getAttribute('data-pass')) || 40;

  var rows = $$('tr[data-row]').map(function (tr) {
    return {
      tr: tr,
      ca: $('input[data-field="ca"]', tr),
      exam: $('input[data-field="exam"]', tr),
      check: $('input[data-select]', tr),
      total: $('[data-total]', tr),
      grade: $('[data-grade]', tr)
    };
  });
  var initial = rows.map(function (r) { return [r.ca.value, r.exam.value]; });
  var submitting = false;

  function grade(t) { return t >= 70 ? 'A' : t >= 60 ? 'B' : t >= 50 ? 'C' : t >= 45 ? 'D' : t >= 40 ? 'E' : 'F'; }
  function round1(n) { return Math.round(n * 10) / 10; }
  function fmt(n) { var s = (Math.round(n * 10) / 10).toFixed(1); return s.replace(/\.0$/, ''); }
  function parse(input, max) {
    var v = input.value.trim();
    if (v === '') return { empty: true };
    var n = Number(v);
    if (!isFinite(n) || n < 0 || n > max) return { bad: true };
    return { n: round1(n) };
  }
  function visible(r) { return !r.tr.hidden; }
  function editable(r) { return !r.ca.disabled && !r.exam.disabled; }

  // ----- live statistics -----
  function update() {
    var totals = [];
    var anyDirty = false;
    rows.forEach(function (r, i) {
      var a = parse(r.ca, MAX.ca), b = parse(r.exam, MAX.exam);
      r.ca.setAttribute('aria-invalid', a.bad ? 'true' : 'false');
      r.exam.setAttribute('aria-invalid', b.bad ? 'true' : 'false');
      if (a.n !== undefined && b.n !== undefined) {
        var t = round1(a.n + b.n);
        totals.push(t);
        r.total.textContent = fmt(t);
        var g = grade(t);
        r.grade.textContent = g;
        r.grade.setAttribute('data-g', g);
      } else {
        r.total.textContent = '';
        r.grade.textContent = '';
        r.grade.removeAttribute('data-g');
      }
      if (r.ca.value !== initial[i][0] || r.exam.value !== initial[i][1]) anyDirty = true;
    });

    var n = totals.length;
    var sorted = totals.slice().sort(function (x, y) { return x - y; });
    var sum = totals.reduce(function (s, v) { return s + v; }, 0);
    var median = n ? (n % 2 ? sorted[(n - 1) / 2] : (sorted[n / 2 - 1] + sorted[n / 2]) / 2) : 0;
    var passed = totals.filter(function (v) { return v >= PASS; }).length;
    var stats = {
      scored: n + ' of ' + rows.length,
      mean: n ? fmt(sum / n) : '-',
      median: n ? fmt(median) : '-',
      max: n ? fmt(sorted[n - 1]) : '-',
      min: n ? fmt(sorted[0]) : '-',
      pass: n ? fmt((passed * 100) / n) + '%' : '-'
    };
    Object.keys(stats).forEach(function (k) {
      $$('[data-stat="' + k + '"]', document).forEach(function (node) { node.textContent = stats[k]; });
    });

    var table = document.getElementById('grade-chart');
    if (table && window.SchoolCharts) {
      var counts = { A: 0, B: 0, C: 0, D: 0, E: 0, F: 0 };
      totals.forEach(function (t) { counts[grade(t)]++; });
      window.SchoolCharts.refresh(table, Object.keys(counts).map(function (k) { return { label: k, value: counts[k], text: String(counts[k]) }; }));
    }

    var dirty = $('[data-dirty]', document);
    if (dirty) {
      if (anyDirty) { dirty.setAttribute('data-on', ''); dirty.textContent = 'You have unsaved changes.'; }
      else { dirty.removeAttribute('data-on'); dirty.textContent = 'All changes are saved.'; }
    }
    form._dirty = anyDirty;
  }

  form.addEventListener('input', function (e) { if (e.target.matches('input.score')) update(); });
  window.addEventListener('beforeunload', function (e) {
    if (form._dirty && !submitting) { e.preventDefault(); e.returnValue = ''; }
  });
  form.addEventListener('submit', function (e) {
    var msgText = e.submitter && e.submitter.getAttribute('data-confirm');
    if (msgText && !window.confirm(msgText)) { e.preventDefault(); return; }
    submitting = true;
  });

  // ----- selecting students -----
  var selectedOut = $('[data-selected]', document);
  var everyoneBtn = $('[data-select-everyone]', document);
  var headCheck = $('input[data-select-all]');
  function selectedRows() { return rows.filter(function (r) { return r.check && r.check.checked && visible(r) && editable(r); }); }
  function refreshSelection() {
    rows.forEach(function (r) { r.tr.classList.toggle('selected', !!(r.check && r.check.checked)); });
    var n = selectedRows().length;
    if (selectedOut) selectedOut.textContent = n === 0 ? 'No students selected' : n + (n === 1 ? ' student selected' : ' students selected');
    var apply = $('[data-bulk-apply]', document);
    if (apply) { apply.disabled = n === 0; apply.textContent = n ? 'Apply to ' + n + (n === 1 ? ' student' : ' students') : 'Apply to selected'; }
    var vis = rows.filter(function (r) { return visible(r) && editable(r); });
    var all = vis.length > 0 && vis.every(function (r) { return r.check.checked; });
    if (everyoneBtn) everyoneBtn.textContent = all ? 'Clear selection' : 'Select everyone' + (vis.length !== rows.length ? ' shown' : '');
    if (headCheck) headCheck.checked = all;
  }
  function setAll(on) { rows.forEach(function (r) { if (r.check && visible(r) && editable(r)) r.check.checked = on; }); refreshSelection(); }
  if (everyoneBtn) everyoneBtn.addEventListener('click', function () {
    var vis = rows.filter(function (r) { return visible(r) && editable(r); });
    setAll(!(vis.length && vis.every(function (r) { return r.check.checked; })));
  });
  if (headCheck) headCheck.addEventListener('change', function () { setAll(headCheck.checked); });
  form.addEventListener('change', function (e) { if (e.target.matches('input[data-select]')) refreshSelection(); });
  form.addEventListener('filtered', refreshSelection);

  // ----- one change for many students -----
  var msg = $('[data-bulk-msg]', document);
  var undoBtn = $('[data-bulk-undo]', document);
  var undoStack = null;
  var apply = $('[data-bulk-apply]', document);
  function say(t) { if (msg) msg.textContent = t; }

  if (apply) apply.addEventListener('click', function () {
    var field = $('[data-bulk-field]', document).value;      // ca | exam
    var op = $('[data-bulk-op]', document).value;            // add | sub | set | clear
    var valueInput = $('[data-bulk-value]', document);
    var amount = Number(valueInput.value);
    var targets = selectedRows();
    if (!targets.length) { say('Select at least one student first.'); return; }
    if (op !== 'clear' && (valueInput.value.trim() === '' || !isFinite(amount) || amount < 0)) {
      say('Enter a number of marks, zero or more.');
      valueInput.focus();
      return;
    }
    var max = MAX[field];
    var changes = [], skipped = 0, clamped = 0;
    targets.forEach(function (r) {
      var input = r[field];
      var prev = input.value;
      var cur = prev.trim() === '' ? null : Number(prev);
      var next;
      if (op === 'clear') next = '';
      else if (op === 'set') next = amount;
      else {
        if (cur === null || !isFinite(cur)) { skipped++; return; }   // never invent a score for a blank cell
        next = op === 'add' ? cur + amount : cur - amount;
      }
      if (next !== '') {
        if (next > max) { next = max; clamped++; }
        if (next < 0) { next = 0; clamped++; }
        next = round1(next);
      }
      changes.push({ input: input, prev: prev });
      input.value = next === '' ? '' : String(next);
    });
    undoStack = changes;
    if (undoBtn) undoBtn.disabled = changes.length === 0;
    update();
    var out = 'Changed ' + changes.length + (changes.length === 1 ? ' score.' : ' scores.');
    if (skipped) out += ' ' + skipped + ' blank ' + (skipped === 1 ? 'score was' : 'scores were') + ' left alone.';
    if (clamped) out += ' ' + clamped + ' kept within 0 to ' + max + '.';
    out += ' Nothing is saved until you press Save draft.';
    say(out);
  });

  if (undoBtn) {
    undoBtn.disabled = true;
    undoBtn.addEventListener('click', function () {
      if (!undoStack) return;
      undoStack.forEach(function (c) { c.input.value = c.prev; });
      say('Last change undone (' + undoStack.length + (undoStack.length === 1 ? ' score).' : ' scores).'));
      undoStack = null;
      undoBtn.disabled = true;
      update();
    });
  }
  var opSelect = $('[data-bulk-op]', document);
  var valueField = $('[data-bulk-value]', document);
  if (opSelect && valueField) {
    var sync = function () { valueField.disabled = opSelect.value === 'clear'; };
    opSelect.addEventListener('change', sync);
    sync();
  }

  // ----- spreadsheet-style keys: Enter and arrows move between students -----
  form.addEventListener('keydown', function (e) {
    var input = e.target;
    if (!input.matches || !input.matches('input.score')) return;
    var dir = e.key === 'Enter' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
    if (!dir) return;
    e.preventDefault();                 // Enter must never submit the whole sheet by accident
    var field = input.getAttribute('data-field');
    var idx = rows.findIndex(function (r) { return r[field] === input; });
    for (var i = idx + dir; i >= 0 && i < rows.length; i += dir) {
      if (visible(rows[i]) && !rows[i][field].disabled) { rows[i][field].focus(); rows[i][field].select(); return; }
    }
  });

  update();
  refreshSelection();
})();
