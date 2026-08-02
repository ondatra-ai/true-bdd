/**
 * TrueBDD harness workspace — mockup niceties (MINIMAL, non-essential).
 *
 * Every mockup page renders correctly with this file absent or JS disabled:
 *   - sidebar sections expand/collapse via native <details>/<summary>;
 *   - CLI prompt dialogs render already-open via <dialog open>.
 *
 * This script only adds small conveniences and is defensive throughout (every
 * handler is wrapped so a DOM shape surprise on one page can never throw and
 * break another page's render or fire a pageerror).
 */
(function () {
  "use strict";

  function onReady(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  onReady(function () {
    // Let a "×" close control inside a prompt dialog dismiss it without
    // requiring a full page reload. Purely a nicety — the dialog is already
    // open via markup, so nothing depends on this running.
    try {
      var closers = document.querySelectorAll("[data-dialog-close]");
      closers.forEach(function (btn) {
        btn.addEventListener("click", function () {
          try {
            var dlg = btn.closest("dialog");
            if (dlg) {
              dlg.removeAttribute("open");
            }
            // The opaque scrim is a sibling element (an attribute-open
            // <dialog> has no ::backdrop), so it must go away with the
            // dialog or the page stays black.
            document.querySelectorAll(".mockup-scrim").forEach(function (scrim) {
              scrim.style.display = "none";
            });
          } catch (innerErr) {
            /* no-op: closing is a nicety, never load-bearing */
          }
        });
      });
    } catch (err) {
      /* no-op: the dialog remains open/inert, which is still correct */
    }

    // Clarify dialog: clicking a numbered option copies its index into the
    // single-line answer input, mirroring the real CLI's "answer with the
    // option number" contract. Purely a nicety.
    try {
      var options = document.querySelectorAll("[data-index]");
      options.forEach(function (opt) {
        opt.addEventListener("click", function () {
          try {
            var input = document.querySelector("#prompt-answer-input");
            if (input) {
              input.value = opt.getAttribute("data-index") || "";
              input.focus();
            }
          } catch (innerErr) {
            /* no-op */
          }
        });
      });
    } catch (err) {
      /* no-op */
    }
  });
})();
