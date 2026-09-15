"""Generate shrimp logo concepts as SVG.

Bodies are built from a centreline plus a width profile so the taper is even;
hand-written beziers wobble at this scale. The silhouette that reads as a
shrimp is one continuous teardrop -- pointed rostrum, heavy carapace, hard
taper into a flared tail fan -- over a sweep of about 220 degrees. More sweep
closes it into a ring, and a separately drawn rostrum reads as a broken
antenna.
"""
import math, os, re

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "drafts")
os.makedirs(OUT, exist_ok=True)

RED = "#DC3535"
ORANGE = "#F97316"
ORANGE_DARK = "#EA580C"


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


def blade(base, ang, length, halfw):
    c, s = math.cos(ang), math.sin(ang)
    tip = (base[0] + length * c, base[1] + length * s)
    mid = (base[0] + length * 0.45 * c, base[1] + length * 0.45 * s)
    nx, ny = -s, c
    l = (mid[0] + nx * halfw, mid[1] + ny * halfw)
    r = (mid[0] - nx * halfw, mid[1] - ny * halfw)
    f = lambda p: "%.1f,%.1f" % p
    return "M%s Q%s %s Q%s %s Z" % (f(base), f(l), f(tip), f(r), f(base))


def fan(base, ang, length, spread=0.56, blades=3, halfw=None, back=0.40):
    """Tail fan. Blades spring from a point set back along the body axis so
    they overlap the abdomen instead of hinging on a visible seam."""
    hw = halfw if halfw is not None else length * 0.29
    root = (base[0] - math.cos(ang) * length * back,
            base[1] - math.sin(ang) * length * back)
    out = []
    for i in range(blades):
        k = (i / (blades - 1)) * 2 - 1 if blades > 1 else 0
        out.append(blade(root, ang + k * spread,
                         length * (1 + back) * (1 - 0.13 * abs(k)), hw))
    return " ".join(out)


def arc_center(cx, cy, a0, a1, r0, r1):
    def fn(t):
        a = math.radians(lerp(a0, a1, t))
        r = lerp(r0, r1, t)
        return (cx + r * math.cos(a), cy + r * math.sin(a))
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


def segments(center_fn, width_fn, ts, grow=1.5):
    out = []
    for t in ts:
        (x, y), (dx, dy), (nx, ny) = sample(center_fn, t)
        w = width_fn(t) * grow
        # Shell joints sit at a slight rake, not square to the spine.
        rx, ry = nx * 0.94 + dx * 0.34, ny * 0.94 + dy * 0.34
        out.append("M%.1f,%.1f L%.1f,%.1f"
                   % (x + rx * w, y + ry * w, x - rx * w, y - ry * w))
    return " ".join(out)


# The carapace-forward teardrop every concept shares.
BODY = profile([(0.00, 1.7), (0.08, 7.2), (0.20, 11.0), (0.38, 9.0),
                (0.62, 6.4), (0.84, 4.0), (1.00, 2.3)])
SLIM = profile([(0.00, 1.5), (0.08, 6.4), (0.20, 9.8), (0.38, 8.0),
                (0.62, 5.7), (0.84, 3.6), (1.00, 2.1)])


def shrimp(uid, center, width, fan_len, fan_kick=0.0, colour=ORANGE,
           segs=(0.36, 0.50, 0.64, 0.78), antennae=True, legs=False,
           eye=True, eye_fill="#FFFFFF", pupil=None, seg_col="#FFFFFF",
           seg_op="0.55", seg_w="2.7"):
    body = cr(taper(center, width))
    (tx, ty), (tdx, tdy), _ = sample(center, 1.0)
    tail = fan((tx, ty), math.atan2(tdy, tdx) + fan_kick, fan_len)
    (hx, hy), (hd), (hn) = sample(center, 0.0)
    fwd = (-hd[0], -hd[1])

    parts = []
    if legs:
        strokes = []
        for t in (0.34, 0.46, 0.58, 0.70):
            (x, y), (dx, dy), (nx, ny) = sample(center, t)
            w = width(t)
            # Swimmerets hang off the belly, which is the inside of the curl.
            strokes.append('M%.1f,%.1f q %.1f,%.1f %.1f,%.1f'
                           % (x - nx * w * 0.55, y - ny * w * 0.55,
                              -nx * 5 + dx * 2.5, -ny * 5 + dy * 2.5,
                              -nx * 7.5 + dx * 6.5, -ny * 7.5 + dy * 6.5))
        parts.append('<g stroke="%s" stroke-width="3" stroke-linecap="round" '
                     'fill="none"><path d="%s"/></g>'
                     % (ORANGE_DARK, " ".join(strokes)))

    if antennae:
        a = ('M%.1f,%.1f q %.1f,%.1f %.1f,%.1f'
             % (hx, hy, fwd[0] * 9 - hn[0] * 2, fwd[1] * 9 - hn[1] * 2,
                fwd[0] * 11 - hn[0] * 13, fwd[1] * 11 - hn[1] * 13))
        b = ('M%.1f,%.1f q %.1f,%.1f %.1f,%.1f'
             % (hx, hy, fwd[0] * 7 + hn[0] * 4, fwd[1] * 7 + hn[1] * 4,
                fwd[0] * 4 + hn[0] * 16, fwd[1] * 4 + hn[1] * 16))
        parts.append('<g stroke="%s" stroke-width="2.8" stroke-linecap="round" '
                     'fill="none"><path d="%s %s"/></g>' % (colour, a, b))

    parts.append('<g fill="%s"><path d="%s"/><path d="%s"/></g>'
                 % (colour, body, tail))

    if segs:
        parts.append('<clipPath id="c-%s"><path d="%s"/></clipPath>' % (uid, body))
        parts.append('<g clip-path="url(#c-%s)" stroke="%s" stroke-width="%s" '
                     'stroke-opacity="%s" stroke-linecap="round">'
                     '<path d="%s"/></g>'
                     % (uid, seg_col, seg_w, seg_op,
                        segments(center, width, segs)))

    if eye:
        (ex, ey), _, (enx, eny) = sample(center, 0.17)
        ew = width(0.17)
        cx_, cy_ = ex + enx * ew * 0.18, ey + eny * ew * 0.18
        parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                     % (cx_, cy_, ew * 0.31, eye_fill))
        if pupil:
            parts.append('<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>'
                         % (cx_ + fwd[0] * 1.1, cy_ + fwd[1] * 1.1,
                            ew * 0.155, pupil))
    return "\n".join(parts)


def svg(name, body, w=100, rotate=None):
    if rotate:
        body = '<g transform="rotate(%s,50,50)">\n%s\n</g>' % (rotate, body)
    doc = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">\n%s\n</svg>\n'
           % (w, w, body))
    # A path missing its slash swallows following siblings as attributes; that
    # silently emptied the clip path once already.
    opens = len(re.findall(r'<(?:path|circle|rect|stop)\b', doc))
    closes = len(re.findall(r'/>', doc))
    assert opens == closes, "%s: %d shapes, %d self-closes" % (name, opens, closes)
    with open(os.path.join(OUT, name + ".svg"), "w") as fh:
        fh.write(doc)
    return doc


CURL = arc_center(50, 46, 207, -16, 26, 25)
DIVE = arc_center(50, 50, 176, 14, 34, 31)


GRAD = ('<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">'
        '<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>'
        '</linearGradient></defs>' % (ORANGE, RED))

# Small sizes lose the antennae and shell joints before they lose the
# silhouette, so the micro cut drops them and fattens what is left.
MICRO = profile([(0.00, 2.2), (0.08, 8.2), (0.20, 12.2), (0.38, 10.2),
                 (0.62, 7.4), (0.84, 4.8), (1.00, 2.8)])


def build():
    svg("curl", shrimp("curl", CURL, BODY, 23, fan_kick=-0.30))
    svg("curl-solid", shrimp("curls", CURL, BODY, 23, fan_kick=-0.30,
                             segs=(), antennae=False))
    svg("mascot", shrimp("mas", CURL, BODY, 24, fan_kick=-0.34, legs=True,
                         pupil=ORANGE_DARK, seg_op="0.45"))
    svg("dive", shrimp("div", DIVE, SLIM, 21, fan_kick=-0.20), rotate=-18)
    svg("curl-grad", GRAD + shrimp("grad", CURL, BODY, 23, fan_kick=-0.30,
                                   colour="url(#g)"))
    svg("curl-micro", shrimp("mic", CURL, MICRO, 26, fan_kick=-0.30,
                             segs=(), antennae=False, eye=False))
    svg("tile", '<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n%s'
        % (RED, shrimp("tile", arc_center(50, 47, 207, -16, 23, 22), SLIM, 20,
                       fan_kick=-0.30, colour="#FFFFFF", eye_fill=RED,
                       seg_col=RED, seg_op="0.85", seg_w="2.4")))


build()
print("\n".join(sorted(os.listdir(OUT))))
