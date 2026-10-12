// Small enhancements used across the portal. Everything works without this file;
// it only makes lists searchable and sortable, and adds copy / print helpers.
(function () {
  'use strict';

  var $ = function (sel, root) { return (root || document).querySelector(sel); };
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); };

  // ----- photographs: if one cannot load, fall back to the plain navy surface -----
  $$('img[data-fallback]').forEach(function (img) {
    var host = img.parentElement;
    var broken = function () { img.remove(); if (host) host.classList.add('no-image'); };
    img.addEventListener('error', broken, { once: true });
    if (img.complete && img.naturalWidth === 0) broken();
  });

  // ----- values for bars and meters come from data attributes (inline styles are blocked by CSP) -----
  $$('[data-w]').forEach(function (n) { n.style.setProperty('--w', n.getAttribute('data-w') + '%'); });
  $$('[data-a]').forEach(function (n) { n.style.setProperty('--a', n.getAttribute('data-a') + '%'); });

  // ----- greeting by time of day -----
  $$('[data-greeting]').forEach(function (el) {
    var h = new Date().getHours();
    el.textContent = h < 12 ? 'Good morning' : h < 17 ? 'Good afternoon' : 'Good evening';
  });

  // ----- live search over a table or list:  <input data-filter="#id"> -----
  $$('input[data-filter]').forEach(function (input) {
    var target = $(input.getAttribute('data-filter'));
    if (!target) return;
    var rows = $$('[data-row]', target);
    var counter = $(input.getAttribute('data-count') || '[data-filter-count]');
    var total = rows.length;
    function run() {
      var q = input.value.trim().toLowerCase();
      var shown = 0;
      rows.forEach(function (row) {
        var match = !q || row.textContent.toLowerCase().indexOf(q) !== -1;
        row.hidden = !match;
        if (match) shown++;
      });
      if (counter) counter.textContent = q ? 'Showing ' + shown + ' of ' + total : total + (total === 1 ? ' student' : ' students');
      target.dispatchEvent(new CustomEvent('filtered', { bubbles: true }));
    }
    input.addEventListener('input', run);
    run();
  });

  // ----- sortable table headers:  <th data-sort="text|num"> -----
  $$('table').forEach(function (table) {
    var heads = $$('th[data-sort]', table);
    if (!heads.length) return;
    heads.forEach(function (th) {
      th.tabIndex = 0;
      th.setAttribute('role', 'columnheader');
      function sort() {
        var idx = th.cellIndex;
        var dir = th.getAttribute('aria-sort') === 'ascending' ? 'descending' : 'ascending';
        heads.forEach(function (h) { h.removeAttribute('aria-sort'); });
        th.setAttribute('aria-sort', dir);
        var numeric = th.getAttribute('data-sort') === 'num';
        var body = table.tBodies[0];
        var rows = $$('tr', body);
        rows.sort(function (a, b) {
          var x = a.cells[idx].getAttribute('data-v') || a.cells[idx].textContent.trim();
          var y = b.cells[idx].getAttribute('data-v') || b.cells[idx].textContent.trim();
          var r = numeric ? (parseFloat(x) || 0) - (parseFloat(y) || 0) : x.localeCompare(y, undefined, { numeric: true, sensitivity: 'base' });
          return dir === 'ascending' ? r : -r;
        });
        rows.forEach(function (r) { body.appendChild(r); });
      }
      th.addEventListener('click', sort);
      th.addEventListener('keydown', function (e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); sort(); } });
    });
  });

  // ----- copy to clipboard:  <button data-copy="#el"> -----
  $$('[data-copy]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var src = $(btn.getAttribute('data-copy'));
      if (!src) return;
      var text = (src.value || src.textContent).trim();
      var done = function () {
        var old = btn.textContent;
        btn.textContent = 'Copied';
        setTimeout(function () { btn.textContent = old; }, 2000);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () { selectText(src); });
      } else { selectText(src); }
    });
  });
  function selectText(el) {
    var range = document.createRange();
    range.selectNodeContents(el);
    var sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  }

  $$('[data-print]').forEach(function (btn) { btn.addEventListener('click', function () { window.print(); }); });

  // ----- ask before destructive actions:  <form data-confirm="Are you sure?"> -----
  $$('form[data-confirm]').forEach(function (form) {
    form.addEventListener('submit', function (e) {
      if (!window.confirm(form.getAttribute('data-confirm'))) e.preventDefault();
    });
  });

  // ----- close the account menu when clicking elsewhere or pressing Escape -----
  var menu = $('details.account');
  if (menu) {
    document.addEventListener('click', function (e) { if (menu.open && !menu.contains(e.target)) menu.open = false; });
    document.addEventListener('keydown', function (e) { if (e.key === 'Escape' && menu.open) { menu.open = false; menu.querySelector('summary').focus(); } });
  }
})();
