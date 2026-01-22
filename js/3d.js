var Atropos = (function () {
  "use strict";
  function B() {
    return (B = Object.assign
      ? Object.assign.bind()
      : function (e) {
          for (var t = 1; t < arguments.length; t++) {
            var i,
              n = arguments[t];
            for (i in n)
              Object.prototype.hasOwnProperty.call(n, i) && (e[i] = n[i]);
          }
          return e;
        }).apply(this, arguments);
  }
  function F(e, t) {
    return e.querySelector(t);
  }
  var N = {
    alwaysActive: !1,
    activeOffset: 50,
    shadowOffset: 50,
    shadowScale: 1,
    duration: 100,
    rotate: !0,
    rotateTouch: !0,
    rotateXMax: 30,
    rotateYMax: 30,
    rotateXInvert: !1,
    rotateYInvert: !1,
    stretchX: 20,
    stretchY: 20,
    stretchZ: 20,
    commonOrigin: !0,
    shadow: !0,
    highlight: !0,
  };
  return function (e) {
    function t(e, t, i, n) {
      e.addEventListener(t, i, n);
    }
    function i(e, t, i, n) {
      e.removeEventListener(t, i, n);
    }
    function n(e) {
      ((v = void 0),
        ("pointerdown" === e.type && "mouse" === e.pointerType) ||
          ("pointerenter" === e.type && "mouse" !== e.pointerType) ||
          ("pointerdown" === e.type && e.preventDefault(),
          (y = e.clientX),
          (b = e.clientY),
          M.alwaysActive
            ? (m = f = void 0)
            : (j(), "function" == typeof M.onEnter && M.onEnter())));
    }
    function r(e) {
      !1 === v && e.cancelable && e.preventDefault();
    }
    function s(e) {
      if (M.rotate && C.isActive) {
        if ("mouse" !== e.pointerType) {
          if (!M.rotateTouch) return;
          e.preventDefault();
        }
        var t = e.clientX,
          i = e.clientY,
          n = t - y,
          r = i - b;
        ("string" != typeof M.rotateTouch ||
          (0 == n && 0 == r) ||
          void 0 !== v ||
          (25 <= n * n + r * r &&
            ((r = (180 * Math.atan2(Math.abs(r), Math.abs(n))) / Math.PI),
            (v = "scroll-y" === M.rotateTouch ? 45 < r : 45 < 90 - r)),
          !1 === v &&
            (T.classList.add("atropos-rotate-touch"), e.cancelable) &&
            e.preventDefault()),
          ("mouse" !== e.pointerType && v) || z(t, i));
      }
    }
    function a(e) {
      ((e = e.target), !S.contains(e) && e !== S && C.isActive && D());
    }
    var c,
      o,
      l,
      h,
      d,
      u,
      f,
      m,
      p,
      g,
      v,
      y,
      b,
      _,
      w,
      x,
      T = (E = e = void 0 === e ? {} : e).el,
      S = E.eventsEl,
      E = e.isComponent,
      C = {
        __atropos__: !0,
        params: B(
          {},
          N,
          { onEnter: null, onLeave: null, onRotate: null },
          (void 0 === (o = e || {}) && (o = {}),
          (l = {}),
          Object.keys(o).forEach(function (e) {
            void 0 !== o[e] && (l[e] = o[e]);
          }),
          l),
        ),
        destroyed: !1,
        isActive: !1,
      },
      M = C.params,
      k = [],
      I =
        ((function e() {
          _ = requestAnimationFrame(function () {
            (k.forEach(function (e) {
              var t, i;
              "function" == typeof e
                ? e()
                : ((t = e.element),
                  (i = e.prop),
                  (e = e.value),
                  (t.style[i] = e));
            }),
              k.splice(0, k.length),
              e());
          });
        })(),
        function (e, t) {
          k.push({ element: e, prop: "transitionDuration", value: t });
        }),
      A = function (e, t) {
        k.push({ element: e, prop: "transitionTimingFunction", value: t });
      },
      O = function (e, t) {
        k.push({ element: e, prop: "transform", value: t });
      },
      L = function (e, t) {
        k.push({ element: e, prop: "opacity", value: t });
      },
      R = function (e, t) {
        k.push({ element: e, prop: "transformOrigin", value: t });
      },
      P = function (e) {
        var t = e.rotateXPercentage,
          r = void 0 === t ? 0 : t,
          t = e.rotateYPercentage,
          s = void 0 === t ? 0 : t,
          a = e.duration,
          o = e.opacityOnly,
          l = e.easeOut;
        ((t = "[data-atropos-offset], [data-atropos-opacity]"),
          c.querySelectorAll(t).forEach(function (e) {
            (I(e, a), A(e, l ? "ease-out" : ""));
            var t,
              i,
              n = (function (e) {
                if (
                  e.dataset.atroposOpacity &&
                  "string" == typeof e.dataset.atroposOpacity
                )
                  return e.dataset.atroposOpacity.split(";").map(function (e) {
                    return parseFloat(e);
                  });
              })(e);
            0 === r && 0 === s
              ? (o || O(e, "translate3d(0, 0, 0)"), n && L(e, n[0]))
              : ((t = parseFloat(e.dataset.atroposOffset) / 100),
                Number.isNaN(t) ||
                  o ||
                  O(e, "translate3d(" + -s * -t + "%, " + r * -t + "%, 0)"),
                n &&
                  ((t = n[0]),
                  (n = n[1]),
                  (i = Math.max(Math.abs(r), Math.abs(s))),
                  L(e, t + ((n - t) * i) / 100)));
          }));
      },
      z = function (e, t) {
        var i,
          n = T !== S,
          r =
            ((f = f || T.getBoundingClientRect()),
            n && !m && (m = S.getBoundingClientRect()),
            void 0 === e &&
              void 0 === t &&
              ((e = (r = n ? m : f).left + r.width / 2),
              (t = r.top + r.height / 2)),
            0),
          s = 0,
          a = f,
          o = a.top,
          l = a.left,
          c = a.width,
          a = a.height,
          d =
            (n
              ? ((u = (p = m).top),
                (i = p.left),
                (d = p.width),
                (p = p.height),
                (s =
                  ((M.rotateYMax * (e - i - (c / 2 + (l - i)))) / (d - c / 2)) *
                  -1),
                (r =
                  (M.rotateXMax * (t - u - (a / 2 + (o - u)))) / (p - a / 2)),
                (i = e - l + "px " + (t - o) + "px"))
              : ((s = ((M.rotateYMax * (e - l - c / 2)) / (c / 2)) * -1),
                (r = (M.rotateXMax * (t - o - a / 2)) / (a / 2))),
            (r = Math.min(Math.max(-r, -M.rotateXMax), M.rotateXMax)),
            M.rotateXInvert && (r = -r),
            (s = Math.min(Math.max(-s, -M.rotateYMax), M.rotateYMax)),
            M.rotateYInvert && (s = -s),
            (r / M.rotateXMax) * 100),
          u = (s / M.rotateYMax) * 100,
          p = (n ? (u / 100) * M.stretchX : 0) * (M.rotateYInvert ? -1 : 1),
          e = (n ? (d / 100) * M.stretchY : 0) * (M.rotateXInvert ? -1 : 1),
          l = n ? (Math.max(Math.abs(d), Math.abs(u)) / 100) * M.stretchZ : 0;
        (O(
          h,
          "translate3d(" +
            p +
            "%, " +
            -e +
            "%, " +
            -l +
            "px) rotateX(" +
            r +
            "deg) rotateY(" +
            s +
            "deg)",
        ),
          i && M.commonOrigin && R(h, i),
          g &&
            (I(g, M.duration + "ms"),
            A(g, "ease-out"),
            O(g, "translate3d(" + 0.25 * -u + "%, " + 0.25 * d + "%, 0)"),
            L(g, Math.max(Math.abs(d), Math.abs(u)) / 100)),
          P({
            rotateXPercentage: d,
            rotateYPercentage: u,
            duration: M.duration + "ms",
            easeOut: !0,
          }),
          "function" == typeof M.onRotate && M.onRotate(r, s));
      },
      j = function () {
        (k.push(function () {
          return T.classList.add("atropos-active");
        }),
          I(h, M.duration + "ms"),
          A(h, "ease-out"),
          O(d, "translate3d(0,0, " + M.activeOffset + "px)"),
          I(d, M.duration + "ms"),
          A(d, "ease-out"),
          p && (I(p, M.duration + "ms"), A(p, "ease-out")),
          (C.isActive = !0));
      },
      D = function (e) {
        ((m = f = void 0),
          !C.isActive ||
            (e && "pointerup" === e.type && "mouse" === e.pointerType) ||
            (e && "pointerleave" === e.type && "mouse" !== e.pointerType) ||
            ("string" == typeof M.rotateTouch &&
              v &&
              T.classList.remove("atropos-rotate-touch"),
            M.alwaysActive
              ? z()
              : (k.push(function () {
                  return T.classList.remove("atropos-active");
                }),
                I(d, M.duration + "ms"),
                A(d, ""),
                O(d, "translate3d(0,0, 0px)"),
                p && (I(p, M.duration + "ms"), A(p, "")),
                g &&
                  (I(g, M.duration + "ms"),
                  A(g, ""),
                  O(g, "translate3d(0, 0, 0)"),
                  L(g, 0)),
                I(h, M.duration + "ms"),
                A(h, ""),
                O(h, "translate3d(0,0,0) rotateX(0deg) rotateY(0deg)"),
                P({ duration: M.duration + "ms" }),
                (C.isActive = !1)),
            "function" == typeof M.onRotate && M.onRotate(0, 0),
            "function" == typeof M.onLeave && M.onLeave()));
      };
    return (
      (C.destroy = function () {
        ((C.destroyed = !0),
          cancelAnimationFrame(_),
          i(document, "click", a),
          i(S, "pointerdown", n),
          i(S, "pointerenter", n),
          i(S, "pointermove", s),
          i(S, "touchmove", r),
          i(S, "pointerleave", D),
          i(S, "pointerup", D),
          i(S, "lostpointercapture", D),
          delete T.__atropos__);
      }),
      (T = "string" == typeof T ? F(document, T) : T) &&
        !T.__atropos__ &&
        (void 0 !== S ? "string" == typeof S && (S = F(document, S)) : (S = T),
        (c = E ? T.parentNode.host : T),
        Object.assign(C, { el: T }),
        (h = F(T, ".atropos-rotate")),
        (d = F(T, ".atropos-scale")),
        (u = F(T, ".atropos-inner")),
        (T.__atropos__ = C)),
      T &&
        S &&
        (M.shadow &&
          ((p = F(T, ".atropos-shadow")) ||
            ((p = document.createElement("span")).classList.add(
              "atropos-shadow",
            ),
            (w = !0)),
          O(
            p,
            "translate3d(0,0,-" +
              M.shadowOffset +
              "px) scale(" +
              M.shadowScale +
              ")",
          ),
          w) &&
          h.appendChild(p),
        M.highlight &&
          ((g = F(T, ".atropos-highlight")) ||
            ((g = document.createElement("span")).classList.add(
              "atropos-highlight",
            ),
            (x = !0)),
          O(g, "translate3d(0,0,0)"),
          x) &&
          u.appendChild(g),
        M.rotateTouch &&
          ("string" == typeof M.rotateTouch
            ? T.classList.add("atropos-rotate-touch-" + M.rotateTouch)
            : T.classList.add("atropos-rotate-touch")),
        F(c, "[data-atropos-opacity]") && P({ opacityOnly: !0 }),
        t(document, "click", a),
        t(S, "pointerdown", n),
        t(S, "pointerenter", n),
        t(S, "pointermove", s),
        t(S, "touchmove", r),
        t(S, "pointerleave", D),
        t(S, "pointerup", D),
        t(S, "lostpointercapture", D),
        M.alwaysActive) &&
        (j(), z()),
      C
    );
  };
})();
