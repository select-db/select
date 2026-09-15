"""Generate the friendly cartoon shrimp, in the existing logo colours.

The palette is the two colours already in web/logo.png: #AC2D31 on #FAFAFA.
Friendliness here is geometry, not styling -- a big forward-facing head with
two eyes and a smile, round caps on every limb, and no point anywhere. The
realistic pointed rostrum is what made the earlier drafts read as seafood
rather than as a character, so it is gone.
"""
import math, os, re

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "drafts")
os.makedirs(OUT, exist_ok=True)

BRAND = "#AC2D31"      # web/logo.png
PAPER = "#FAFAFA"      # web/logo.png ground
DARK = "#7C1E22"       # pupils and shadow, same hue
TINT = "#C7484C"       # shell joints and belly, same hue


def lerp(a, b, t):
    return a + (b - a) * t


def sample(fn, t, eps=1e-4):
    x, y = fn(t)
    x1, y1 = fn(max(t - eps, 0.0))
    x2, y2 = fn(min(t + eps, 1.0))
    dx, dy = x2 - x1, y2 - y1
    L = math.hypot(dx, dy) or 1.0
    return (x, y), (dx / L, dy / L), (-dy / L, dx / L)


def taper(center_fn, width_fn, n=34):
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


def arc_center(cx, cy, a0, a1, r0, r1):
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


def stroke(d, colour, w, extra=""):
    return ('<path d="%s" fill="none" stroke="%s" stroke-width="%.1f" '
            'stroke-linecap="round" stroke-linejoin="round"%s/>'
            % (d, colour, w, extra))


# Body: starts fat where the head sits over it, tapers hard to the tail base.
SPINE = arc_center(50, 46, 207, -16, 26, 25)
GIRTH = profile([(0.00, 12.0), (0.15, 12.2), (0.35, 10.0),
                 (0.60, 7.2), (0.82, 5.4), (1.00, 4.2)])
CHUNK = profile([(0.00, 13.0), (0.15, 13.2), (0.35, 11.2),
                 (0.60, 8.4), (0.82, 6.4), (1.00, 5.0)])

HEAD = SPINE(0.0)
HEAD_R = 12.3


def face(palette, wink=False):
    """Two forward-facing eyes and a smile, placed off the head centre. The
    size difference between them is what turns a flat front view into a
    three-quarter one; equal circles read as a bug."""
    colour, paper, dark = palette["body"], palette["paper"], palette["dark"]
    hx, hy = HEAD
    lx, ly, lr = hx - 4.8, hy - 6.2, 5.7
    rx, ry, rr = hx + 5.4, hy - 6.9, 5.2
    out = ['<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>' % (lx, ly, lr, paper),
           '<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>' % (rx, ry, rr, paper)]
    out.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
               % (lx + 1.5, ly + 1.0, lr * 0.46, dark))
    if wink:
        out.append(stroke("M%.1f,%.1f q 3.4,-3.2 6.8,0.2" % (rx - 3.4, ry + 0.6), dark, 2.4))
    else:
        out.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                   % (rx + 1.3, ry + 0.9, rr * 0.46, dark))
    out.append(stroke("M%.1f,%.1f q 4.6,4.8 9.2,0.2" % (hx - 4.6, hy + 3.4), dark, 2.9))
    return "\n".join(out)


def cartoon(uid, girth=GIRTH, fan_len=23, legs=True, arms=True, antennae=True,
            joints=True, eyes=True, wink=False, wave=False,
            body_col=BRAND, paper_col=PAPER, dark_col=DARK, tint_col=TINT):
    palette = {"body": body_col, "paper": paper_col, "dark": dark_col}
    body = cr(taper(SPINE, girth))
    (tx, ty), (tdx, tdy), _ = sample(SPINE, 1.0)
    ang = math.atan2(tdy, tdx) - 0.36
    root = SPINE(0.90)
    hx, hy = HEAD

    parts = []
    if legs:
        # Swimmerets point into the open side of the curl, which is the only
        # place they are not hidden behind the body.
        d = []
        for t in (0.34, 0.50, 0.66):
            (x, y), _, (nx, ny) = sample(SPINE, t)
            w = girth(t)
            d.append(ray((x - nx * w * 0.55, y - ny * w * 0.55),
                         math.atan2(-ny, -nx) + 0.40, 7.0))
        parts.append(stroke(" ".join(d), body_col, 4.4))

    if antennae:
        parts.append(stroke(
            "M%.1f,%.1f q -5.5,-10.5 -13.0,-13.5 M%.1f,%.1f q 6.0,-10.0 14.5,-11.5"
            % (hx - 4.5, hy - 9.5, hx + 5.0, hy - 9.0), body_col, 3.2))

    # Round-capped strokes rather than pointed leaves: no spikes anywhere.
    parts.append(stroke(
        " ".join(ray(root, ang + k * 0.46, fan_len * (1 - 0.08 * abs(k)))
                 for k in (-1, 0, 1)), body_col, fan_len * 0.34))

    parts.append('<path d="%s" fill="%s"/>' % (body, body_col))
    parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                 % (hx, hy, HEAD_R, body_col))

    if arms:
        parts.append(stroke("M%.1f,%.1f q 9.5,-2.0 10.5,-10.5" % (hx + 8.0, hy + 8.5),
                            body_col, 5.2))

    if joints:
        parts.append('<clipPath id="j-%s"><path d="%s"/></clipPath>' % (uid, body))
        d = []
        for t in (0.50, 0.65, 0.80):
            (x, y), (dx, dy), (nx, ny) = sample(SPINE, t)
            w = girth(t) * 1.6
            rx, ry = nx * 0.94 + dx * 0.34, ny * 0.94 + dy * 0.34
            d.append("M%.1f,%.1f L%.1f,%.1f"
                     % (x + rx * w, y + ry * w, x - rx * w, y - ry * w))
        parts.append('<g clip-path="url(#j-%s)">%s</g>'
                     % (uid, stroke(" ".join(d), tint_col, 3.0)))

    if eyes:
        parts.append(face(palette, wink=wink))
    return "\n".join(parts)


def svg(name, body, w=100, pre=""):
    doc = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">\n%s%s\n</svg>\n'
           % (w, w, pre, body))
    opens = len(re.findall(r'<(?:path|circle|rect|stop)\b', doc))
    assert opens == len(re.findall(r'/>', doc)), name
    open(os.path.join(OUT, name + ".svg"), "w").write(doc)
    return doc


svg("cartoon", cartoon("a", legs=False, arms=False))
svg("cartoon-wave", cartoon("b", legs=False, arms=True, wink=True))
svg("cartoon-legs", cartoon("c", legs=True, arms=False))
svg("cartoon-micro", cartoon("d", girth=CHUNK, fan_len=26, legs=False,
                             arms=False, antennae=False, joints=False))
svg("cartoon-tile", cartoon("e"),
    pre='<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n' % PAPER)
svg("cartoon-invert", cartoon("f", body_col=PAPER, paper_col=BRAND,
                              dark_col=BRAND, tint_col="#8E2328"),
    pre='<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n' % BRAND)
print("\n".join(sorted(os.listdir(OUT))))
