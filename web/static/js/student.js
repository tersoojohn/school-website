// Student area animations. One orchestrated entrance: the average ring draws, its number
// counts up, subject bars fill in order and each grade is stamped. Rows expand on click.
// With reduced motion (or no JavaScript) the final state is shown straight away.
(function () {
  'use strict';
  var reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); };

  // Per-element values come from data attributes (inline styles are not allowed by the site's CSP).
  $$('[data-d]').forEach(function (n) { n.style.setProperty('--d', n.getAttribute('data-d')); });
  $$('[data-i]').forEach(function (n) { n.style.setProperty('--i', n.getAttribute('data-i')); });
  $$('[data-w]').forEach(function (n) { n.style.setProperty('--w', n.getAttribute('data-w') + '%'); });
  $$('[data-a]').forEach(function (n) { n.style.setProperty('--a', n.getAttribute('data-a') + '%'); });
  $$('[data-ring]').forEach(function (n) {
    var v = parseFloat(n.getAttribute('data-ring')) || 0;
    n.style.setProperty('--p', Math.max(0, Math.min(1, v / 100)));
  });

  function countUp(node) {
    var raw = node.getAttribute('data-count');
    var target = parseFloat(raw);
    if (isNaN(target)) return;
    var decimals = (raw.split('.')[1] || '').length;
    if (reduce) { node.textContent = target.toFixed(decimals); return; }
    node.textContent = (0).toFixed(decimals);
    var start = null, delay = 350, dur = 1500;
    function frame(ts) {
      if (start === null) start = ts;
      var t = Math.min(1, Math.max(0, (ts - start - delay) / dur));
      var eased = 1 - Math.pow(1 - t, 3);
      node.textContent = (target * eased).toFixed(decimals);
      if (t < 1) requestAnimationFrame(frame);
    }
    requestAnimationFrame(frame);
  }

  function enter(node) {
    if (node.classList.contains('is-in')) return;
    node.classList.add('is-in');
    $$('[data-count]', node).forEach(countUp);
    if (node.hasAttribute('data-count')) countUp(node);
  }

  var targets = $$('[data-enter], .reveal');
  if ('IntersectionObserver' in window && !reduce) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) { enter(e.target); io.unobserve(e.target); }
      });
    }, { threshold: 0.18 });
    targets.forEach(function (t) { io.observe(t); });
  } else {
    targets.forEach(enter);
  }

  // Expandable subject rows
  $$('.subject-row').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var li = btn.closest('.subject');
      var open = li.classList.toggle('open');
      btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
  });
  var firstOpen = document.querySelector('.subject-row[data-open-first]');
  if (firstOpen) setTimeout(function () { firstOpen.click(); }, reduce ? 0 : 2400);
})();
