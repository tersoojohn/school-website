// Turns <table class="chart-data" data-chart="bars|columns|line"> into a chart.
// The table stays in the page for screen readers (and for anyone without scripts).
(function () {
  'use strict';

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function read(table) {
    return Array.prototype.map.call(table.tBodies[0].rows, function (tr) {
      var cell = tr.cells[1];
      var raw = cell.getAttribute('data-v');
      var v = parseFloat(raw != null ? raw : cell.textContent);
      return { label: tr.cells[0].textContent.trim(), value: isNaN(v) ? 0 : v, text: cell.textContent.trim() };
    });
  }

  function bars(items, opts) {
    var wrap = el('div', 'cbars');
    var max = opts.max || Math.max.apply(null, items.map(function (i) { return i.value; }).concat([1]));
    items.forEach(function (it) {
      var row = el('div', 'cbar');
      row.appendChild(el('span', 'cbar-label', it.label)).title = it.label;
      var track = el('span', 'cbar-track');
      var fill = el('span', 'cbar-fill');
      fill.style.setProperty('--w', Math.min(100, (it.value / max) * 100) + '%');
      track.appendChild(fill);
      row.appendChild(track);
      row.appendChild(el('span', 'cbar-value', it.text));
      wrap.appendChild(row);
    });
    return wrap;
  }

  function columns(items, opts) {
    var wrap = el('div');
    var cols = el('div', 'ccols');
    var labels = el('div', 'cxlabels');
    var max = Math.max.apply(null, items.map(function (i) { return i.value; }).concat([1]));
    var bad = (opts.bad || '').split(',');
    items.forEach(function (it) {
      var c = el('div', 'ccol');
      if (bad.indexOf(it.label) !== -1) c.setAttribute('data-bad', '');
      if (it.value === 0) c.setAttribute('data-zero', '');
      c.appendChild(el('span', 'ccol-value', it.text));
      var b = el('span', 'ccol-bar');
      b.style.setProperty('--h', (it.value / max) * 100 + '%');
      c.appendChild(b);
      cols.appendChild(c);
      labels.appendChild(el('span', '', it.label));
    });
    wrap.appendChild(cols);
    wrap.appendChild(labels);
    return wrap;
  }

  function line(items, opts) {
    var NS = 'http://www.w3.org/2000/svg';
    var W = 560, H = 220, padL = 40, padR = 40, padT = 28, padB = 36;
    var lo = opts.min != null ? opts.min : 0, hi = opts.max != null ? opts.max : 100;
    var svg = document.createElementNS(NS, 'svg');
    svg.setAttribute('viewBox', '0 0 ' + W + ' ' + H);
    svg.setAttribute('role', 'presentation');
    function mk(name, attrs) {
      var n = document.createElementNS(NS, name);
      for (var k in attrs) n.setAttribute(k, attrs[k]);
      return n;
    }
    var y = function (v) { return padT + (1 - (v - lo) / (hi - lo)) * (H - padT - padB); };
    var x = function (i) { return items.length === 1 ? W / 2 : padL + (i * (W - padL - padR)) / (items.length - 1); };
    [lo, (lo + hi) / 2, hi].forEach(function (t) {
      svg.appendChild(mk('line', { class: 'axis', x1: padL, x2: W - padR, y1: y(t), y2: y(t) }));
      var tx = mk('text', { class: 'tick', x: padL - 8, y: y(t) + 4, 'text-anchor': 'end' });
      tx.textContent = Math.round(t);
      svg.appendChild(tx);
    });
    if (items.length > 1) {
      svg.appendChild(mk('polyline', { class: 'path', points: items.map(function (it, i) { return x(i) + ',' + y(it.value); }).join(' ') }));
    }
    items.forEach(function (it, i) {
      svg.appendChild(mk('circle', { class: 'dot', cx: x(i), cy: y(it.value), r: 5 }));
      var v = mk('text', { x: x(i), y: y(it.value) - 12, 'text-anchor': 'middle' });
      v.textContent = it.text;
      svg.appendChild(v);
      var anchor = items.length > 1 ? (i === 0 ? 'start' : i === items.length - 1 ? 'end' : 'middle') : 'middle';
      var lx = anchor === 'start' ? padL - 10 : anchor === 'end' ? W - padR + 10 : x(i);
      var l = mk('text', { class: 'tick', x: lx, y: H - 10, 'text-anchor': anchor });
      l.textContent = it.label.length > 22 ? it.label.slice(0, 21) + '\u2026' : it.label;
      svg.appendChild(l);
    });
    var wrap = el('div', 'cline');
    wrap.appendChild(svg);
    return wrap;
  }

  var kinds = { bars: bars, columns: columns, line: line };

  function render(table, items) {
    var kind = table.getAttribute('data-chart');
    var opts = {
      max: table.hasAttribute('data-max') ? parseFloat(table.getAttribute('data-max')) : null,
      min: table.hasAttribute('data-min') ? parseFloat(table.getAttribute('data-min')) : null,
      bad: table.getAttribute('data-bad')
    };
    var viz = kinds[kind](items, opts);
    viz.setAttribute('aria-hidden', 'true');
    if (table._viz) table._viz.replaceWith(viz); else table.parentNode.insertBefore(viz, table);
    table._viz = viz;
    table.classList.add('enhanced');
  }

  function init() {
    Array.prototype.forEach.call(document.querySelectorAll('table[data-chart]'), function (t) {
      if (!kinds[t.getAttribute('data-chart')] || !t.tBodies[0] || !t.tBodies[0].rows.length) return;
      render(t, read(t));
    });
  }

  // Used by the score sheet to redraw the grade chart as marks are typed.
  window.SchoolCharts = {
    refresh: function (table, items) { if (table) render(table, items); }
  };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
