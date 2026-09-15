"""Generate the shrimp variants, in the existing logo colours.

Everything is one parametric character. The axes that matter are the spine
sweep (how closed the curl is), the girth profile (weight), the sample count
(silhouette complexity, which is what decides small-size legibility) and how
much face and how many limbs are kept.

Sample count is the cheap lever. cr() emits one cubic per sample, so n=34
costs 70 curve commands before anything else is drawn. Comparable dev-tool
marks reduced to a single silhouette run 3 commands (Vercel) to 194 (Rust),
with DBeaver at 39 and Bruno at 70; n=11 puts the reduced cut in that range
at no visible cost.
"""
import math, os, re

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "drafts")
os.makedirs(OUT, exist_ok=True)

BRAND = "#AC2D31"      # web/logo.png
PAPER = "#FAFAFA"      # web/logo.png ground
DARK = "#7C1E22"       # pupils and smile, same hue
TINT = "#C7484C"       # shell joints, same hue


def lerp(a, b, t):
    return a + (b - a) * t


def sample(fn, t, eps=1e-4):
    x, y = fn(t)
    x1, y1 = fn(max(t - eps, 0.0))
    x2, y2 = fn(min(t + eps, 1.0))
    dx, dy = x2 - x1, y2 - y1
    L = math.hypot(dx, dy) or 1.0
    return (x, y), (dx / L, dy / L), (-dy / L, dx / L)


def taper(center_fn, width_fn, n):
    left, right = [], []
    for i in range(n + 1):
        t = i / n
        (x, y), _, (nx, ny) = sample(center_fn, t)
        w = width_fn(t)
        left.append((x + nx * w, y + ny * w))
        right.append((x - nx * w, y - ny * w))
    return left + right[::-1]


def cr(points):
    n = len(points)
    f = lambda p: "%.1f,%.1f" % p
    d = "M" + f(points[0])
    for i in range(n):
        p0, p1 = points[(i - 1) % n], points[i % n]
        p2, p3 = points[(i + 1) % n], points[(i + 2) % n]
        c1 = (p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6)
        c2 = (p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6)
        d += "C%s %s %s" % (f(c1), f(c2), f(p2))
    return d + "Z"


def arc(cx, cy, a0, a1, r0, r1):
    def fn(t):
        a = math.radians(lerp(a0, a1, t))
        return (cx + lerp(r0, r1, t) * math.cos(a),
                cy + lerp(r0, r1, t) * math.sin(a))
    return fn


def profile(stops):
    def fn(t):
        for i in range(len(stops) - 1):
            t0, w0 = stops[i]
            t1, w1 = stops[i + 1]
            if t0 <= t <= t1:
                return lerp(w0, w1, (t - t0) / (t1 - t0))
        return stops[-1][1]
    return fn


def ray(p, ang, length):
    return "M%.1f,%.1f L%.1f,%.1f" % (
        p[0], p[1], p[0] + length * math.cos(ang), p[1] + length * math.sin(ang))


def stroke(d, colour, w):
    return ('<path d="%s" fill="none" stroke="%s" stroke-width="%.1f" '
            'stroke-linecap="round" stroke-linejoin="round"/>' % (d, colour, w))


LIGHT = profile([(0.00, 10.4), (0.15, 10.6), (0.35, 8.8),
                 (0.60, 6.4), (0.82, 4.6), (1.00, 3.6)])
GIRTH = profile([(0.00, 12.0), (0.15, 12.2), (0.35, 10.0),
                 (0.60, 7.2), (0.82, 5.4), (1.00, 4.2)])
CHUNK = profile([(0.00, 13.0), (0.15, 13.2), (0.35, 11.2),
                 (0.60, 8.4), (0.82, 6.4), (1.00, 5.0)])


def build(uid, a0=207, a1=-16, r0=26, r1=25, cx=50, cy=46, girth=GIRTH,
          head_r=12.3, n=34, fan_len=23, kick=-0.36, face="full",
          antennae=True, legs=False, wave=False, joints=True,
          body_col=BRAND, paper_col=PAPER, dark_col=DARK, tint_col=TINT):
    spine = arc(cx, cy, a0, a1, r0, r1)
    body = cr(taper(spine, girth, n))
    (tx, ty), (tdx, tdy), _ = sample(spine, 1.0)
    ang = math.atan2(tdy, tdx) + kick
    root = spine(0.90)
    hx, hy = spine(0.0)

    parts = []
    if legs:
        # Swimmerets point into the open side of the curl, the only place
        # they are not hidden behind the body.
        d = []
        for t in (0.34, 0.50, 0.66):
            (x, y), _, (nx, ny) = sample(spine, t)
            w = girth(t)
            d.append(ray((x - nx * w * 0.55, y - ny * w * 0.55),
                         math.atan2(-ny, -nx) + 0.40, 7.0))
        parts.append(stroke(" ".join(d), body_col, 4.4))

    if antennae:
        parts.append(stroke(
            "M%.1f,%.1f q -5.5,-10.5 -13.0,-13.5 M%.1f,%.1f q 6.0,-10.0 14.5,-11.5"
            % (hx - 4.5, hy - 9.5, hx + 5.0, hy - 9.0), body_col, 3.2))

    parts.append(stroke(
        " ".join(ray(root, ang + k * 0.46, fan_len * (1 - 0.08 * abs(k)))
                 for k in (-1, 0, 1)), body_col, fan_len * 0.34))
    parts.append('<path d="%s" fill="%s"/>' % (body, body_col))
    parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                 % (hx, hy, head_r, body_col))

    if wave:
        parts.append(stroke("M%.1f,%.1f q 9.5,-2.0 10.5,-10.5" % (hx + 8.0, hy + 8.5),
                            body_col, 5.2))

    if joints:
        parts.append('<clipPath id="j-%s"><path d="%s"/></clipPath>' % (uid, body))
        d = []
        for t in (0.50, 0.65, 0.80):
            (x, y), (dx, dy), (nx, ny) = sample(spine, t)
            w = girth(t) * 1.6
            rx, ry = nx * 0.94 + dx * 0.34, ny * 0.94 + dy * 0.34
            d.append("M%.1f,%.1f L%.1f,%.1f"
                     % (x + rx * w, y + ry * w, x - rx * w, y - ry * w))
        parts.append('<g clip-path="url(#j-%s)">%s</g>'
                     % (uid, stroke(" ".join(d), tint_col, 3.0)))

    if face != "none":
        s = head_r / 12.3
        lx, ly, lr = hx - 4.8 * s, hy - 6.2 * s, 5.7 * s
        rx, ry, rr = hx + 5.4 * s, hy - 6.9 * s, 5.2 * s
        parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                     % (lx, ly, lr, paper_col))
        parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                     % (rx, ry, rr, paper_col))
        parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                     % (lx + 1.5 * s, ly + 1.0 * s, lr * 0.46, dark_col))
        parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                     % (rx + 1.3 * s, ry + 0.9 * s, rr * 0.46, dark_col))
        if face == "full":
            parts.append(stroke("M%.1f,%.1f q %.1f,%.1f %.1f,%.1f"
                                % (hx - 4.6 * s, hy + 3.4 * s, 4.6 * s, 4.8 * s,
                                   9.2 * s, 0.2 * s), dark_col, 2.9))
    return "\n".join(parts)


def svg(name, body, pre="", post="", mirror=False, w=100):
    if mirror:
        body = '<g transform="translate(%d,0) scale(-1,1)">\n%s\n</g>' % (w, body)
    doc = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">\n%s%s%s\n</svg>\n'
           % (w, w, pre, body, post))
    assert (len(re.findall(r'<(?:path|circle|rect|stop)\b', doc))
            == len(re.findall(r'/>', doc))), name
    open(os.path.join(OUT, name + ".svg"), "w").write(doc)


TILE = '<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n'

# --- curl family: the mark -------------------------------------------------
svg("curl", build("a", legs=False))
svg("curl-reduced", build("b", n=11, face="dots", antennae=False, joints=False,
                          girth=CHUNK, fan_len=25))
svg("curl-open", build("c", a0=196, a1=22, r0=27, r1=26, girth=LIGHT, fan_len=21))
svg("curl-tight", build("d", a0=218, a1=-42, r0=25, r1=23, girth=CHUNK, fan_len=22))
svg("curl-badge", build("e", n=14, face="dots", antennae=False, joints=False,
                        girth=CHUNK, fan_len=24, body_col=PAPER, paper_col=BRAND),
    pre='<circle cx="50" cy="50" r="48" fill="%s"/>\n' % BRAND)
svg("curl-mirror", build("f", legs=False), mirror=True)

# --- mascot family: the character ------------------------------------------
svg("mascot", build("g", legs=True))
svg("mascot-wave", build("h", legs=True, wave=True))
svg("mascot-chibi", build("i", a0=210, a1=-30, r0=23, r1=22, head_r=15.5,
                          girth=CHUNK, fan_len=22, legs=True))
svg("mascot-peek",
    build("j", cy=60, legs=False, joints=False),
    pre='<clipPath id="peek"><rect x="0" y="0" width="100" height="72"/></clipPath>\n<g clip-path="url(#peek)">',
    post='</g>\n<rect x="6" y="68" width="88" height="9" rx="4.5" fill="%s"/>' % TINT)

# --- tiles -----------------------------------------------------------------
svg("tile-red", build("k", n=14, face="dots", antennae=False, joints=False,
                      girth=CHUNK, fan_len=24, body_col=PAPER, paper_col=BRAND),
    pre=TILE % BRAND)
svg("tile-paper", build("l", legs=False), pre=TILE % PAPER)

print("\n".join(sorted(os.listdir(OUT))))
