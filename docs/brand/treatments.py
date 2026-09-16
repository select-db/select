"""Treatments on the rounded bowtie.

One geometry, many surfaces. The flat version stays the master -- gradients,
extrusion and grain all collapse at favicon size -- so these are for the hero,
the app icon and marketing, not for the 16px slot.

The overlap treatment is the one that comes out of this mark specifically:
the two triangles genuinely cross, so the crossing region is computed with a
Sutherland-Hodgman clip and shaded, rather than faked with a blend mode that
renderers disagree about.
"""
import math, os, re

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "drafts-treatments")
os.makedirs(OUT, exist_ok=True)

BRAND = "#AC2D31"
PAPER = "#FAFAFA"
DEEP = "#7C1E22"
DARKEST = "#5A1417"
WARM = "#D8542F"
TINT = "#C7484C"

TOP = [(10, 10), (90, 10), (38.6, 61.6)]
BOT = [(90, 90), (10, 90), (61.4, 38.4)]
R = 10.0


def poly(points):
    return "M%.1f,%.1f" % points[0] + "".join("L%.1f,%.1f" % p for p in points[1:]) + "Z"


def scale(points, k, cx=50, cy=50):
    return [(cx + (x - cx) * k, cy + (y - cy) * k) for x, y in points]


def shift(points, dx, dy):
    return [(x + dx, y + dy) for x, y in points]


def rot180(p, cx=50, cy=50):
    return (2 * cx - p[0], 2 * cy - p[1])


def area(points):
    n = len(points)
    return sum(points[i][0] * points[(i + 1) % n][1] - points[(i + 1) % n][0] * points[i][1]
               for i in range(n)) / 2


def clip(subject, clipper):
    """Sutherland-Hodgman. Both polygons here are convex, so the result is
    exact rather than an approximation."""
    s = 1 if area(clipper) > 0 else -1

    def inside(p, a, b):
        return s * ((b[0] - a[0]) * (p[1] - a[1]) - (b[1] - a[1]) * (p[0] - a[0])) >= 0

    def cross(p1, p2, a, b):
        (x1, y1), (x2, y2), (x3, y3), (x4, y4) = p1, p2, a, b
        den = (x1 - x2) * (y3 - y4) - (y1 - y2) * (x3 - x4)
        if abs(den) < 1e-9:
            return p2
        t = ((x1 - x3) * (y3 - y4) - (y1 - y3) * (x3 - x4)) / den
        return (x1 + t * (x2 - x1), y1 + t * (y2 - y1))

    out = list(subject)
    n = len(clipper)
    for i in range(n):
        a, b = clipper[i], clipper[(i + 1) % n]
        src, out = out, []
        if not src:
            break
        for j in range(len(src)):
            cur, prv = src[j], src[j - 1]
            if inside(cur, a, b):
                if not inside(prv, a, b):
                    out.append(cross(prv, cur, a, b))
                out.append(cur)
            elif inside(prv, a, b):
                out.append(cross(prv, cur, a, b))
    return out


def rnd(points, fill_col, r=R, span=40.0, stroke_col=None, extra=""):
    """Rounded corners from a round-joined stroke of the fill colour; the
    polygon is pre-scaled so the stroke lands back on the original extent."""
    c = stroke_col or fill_col
    return ('<path d="%s" fill="%s" stroke="%s" stroke-width="%.1f" '
            'stroke-linejoin="round"%s/>'
            % (poly(scale(points, 1 - r / (2 * span))), fill_col, c, r, extra))


def mark(fill_col=BRAND, r=R, extra=""):
    return rnd(TOP, fill_col, r, extra=extra) + rnd(BOT, fill_col, r, extra=extra)


def svg(name, body, defs="", w=100):
    doc = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">\n%s%s\n</svg>\n'
           % (w, w, defs, body))
    assert (len(re.findall(r'<(?:path|circle|rect|stop|feDropShadow|feTurbulence'
                           r'|feColorMatrix|feComposite|feGaussianBlur)\b', doc))
            == len(re.findall(r'/>', doc))), name
    open(os.path.join(OUT, name + ".svg"), "w").write(doc)


def lin(uid, a, b, x2=1, y2=1):
    return ('<defs><linearGradient id="%s" x1="0" y1="0" x2="%s" y2="%s">'
            '<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>'
            '</linearGradient></defs>\n' % (uid, x2, y2, a, b))


SQ = '<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n'

# 01 -- the master. Everything below is this shape wearing something.
svg("flat", mark())

# 02 -- linear gradient, staying inside the brand hue.
svg("grad", mark("url(#g)"), lin("g", BRAND, DARKEST))

# 03 -- the same move with a warm push at the top. The only variant that
#       introduces a hue the brand does not already own.
svg("grad-warm", mark("url(#w)"), lin("w", WARM, BRAND, x2=0.3, y2=1))

# 04 -- radial, lit from upper left.
svg("grad-radial", mark("url(#r)"),
    '<defs><radialGradient id="r" cx="0.3" cy="0.2" r="0.95">'
    '<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>'
    '</radialGradient></defs>\n' % (TINT, DEEP))

# 05 -- the crossing region shaded. Specific to this mark: the triangles
#       really do cross, so the crossing is drawn rather than faked with a
#       blend mode. The apexes reach further than on the master, because at
#       the original depth the intersection is a sliver that reads as a
#       printing fault rather than as a decision.
DEEP_TOP = [(10, 10), (90, 10), (33, 74)]
DEEP_BOT = [rot180(p) for p in DEEP_TOP]
svg("overlap",
    rnd(DEEP_TOP, BRAND) + rnd(DEEP_BOT, BRAND)
    + rnd(clip(DEEP_TOP, DEEP_BOT), DARKEST, r=8, span=22))

# 06 -- stroke only.
svg("outline", mark("none", extra=' stroke="%s"' % BRAND).replace(
    'fill="none" stroke="none"', 'fill="none"'))

# 07 -- one solid, one hollow, so the two triangles read as two.
svg("duo", rnd(TOP, BRAND) + rnd(BOT, "none", stroke_col=BRAND, r=7))

# 08 -- die-cut sticker: a paper keyline and a soft drop.
svg("sticker", '<g filter="url(#d)">%s%s</g>' % (mark(PAPER, r=22), mark(BRAND)),
    '<defs><filter id="d" x="-20%" y="-20%" width="140%" height="140%">'
    '<feDropShadow dx="0" dy="2.5" stdDeviation="2.5" flood-color="#5A1417" '
    'flood-opacity="0.3"/></filter></defs>\n')

# 09 -- cartoon: a heavy dark keyline and a flat offset shadow underneath.
svg("cartoon",
    "".join(rnd(shift(p, 2.5, 3), DARKEST) for p in (TOP, BOT))
    + mark(DARKEST, r=17) + mark(BRAND))

# 10 -- extruded. Stacked offsets rather than a filter, so it stays vector.
svg("extrude",
    "".join(rnd(shift(p, i * 0.9, i * 0.9), DARKEST)
            for i in range(7, 0, -1) for p in (TOP, BOT))
    + mark("url(#e)"), lin("e", TINT, BRAND))

# 11 -- grain, the texture everything has this year. Turbulence clipped to
#       the mark and multiplied over a flat fill.
svg("grain",
    mark(BRAND) + '<g clip-path="url(#cg)"><rect width="100" height="100" '
    'filter="url(#n)" opacity="0.5" style="mix-blend-mode:multiply"/></g>',
    '<defs><filter id="n"><feTurbulence type="fractalNoise" baseFrequency="0.85" '
    'numOctaves="4"/><feColorMatrix type="saturate" values="0"/></filter>'
    '<clipPath id="cg">%s</clipPath></defs>\n'
    % (poly(scale(TOP, 0.875)) and
       '<path d="%s"/><path d="%s"/>' % (poly(scale(TOP, 1.02)), poly(scale(BOT, 1.02)))))

# 12 -- frosted glass over a gradient ground; needs a backdrop to read at all.
#       No inner highlight: on a shape this angular it reads as a scratch.
svg("glass",
    '<rect x="2" y="2" width="96" height="96" rx="24" fill="url(#gg)"/>\n'
    + mark("#FFFFFF", extra=' fill-opacity="0.20" stroke-opacity="0.42"')
    + rnd(scale(TOP, 0.92), "url(#sheen)", extra=' stroke="none"'),
    lin("gg", BRAND, DARKEST)
    + '<defs><linearGradient id="sheen" x1="0" y1="0" x2="0" y2="1">'
      '<stop offset="0" stop-color="#FFFFFF" stop-opacity="0.32"/>'
      '<stop offset="1" stop-color="#FFFFFF" stop-opacity="0"/>'
      '</linearGradient></defs>\n')

# 13 -- the app icon, with the tile carrying the gradient and the mark flat.
svg("tile-grad",
    '<rect x="2" y="2" width="96" height="96" rx="24" fill="url(#tg)"/>\n'
    + rnd(scale(TOP, 0.66), PAPER, r=7, span=26)
    + rnd(scale(BOT, 0.66), PAPER, r=7, span=26),
    lin("tg", TINT, DEEP))

# 14 -- inverted sticker for dark chrome.
svg("tile-outline",
    SQ % BRAND + rnd(scale(TOP, 0.66), "none", r=6, span=26, stroke_col=PAPER)
    + rnd(scale(BOT, 0.66), "none", r=6, span=26, stroke_col=PAPER))


def commands(path):
    s = open(path).read()
    return (sum(len(re.findall(r'[MmLlHhVvCcSsQqTtAaZz]', d))
                for d in re.findall(r'\sd="([^"]*)"', s))
            + len(re.findall(r'<(?:rect|circle)\b', s)))


for f in sorted(os.listdir(OUT)):
    print("%-13s %3d commands" % (f[:-4], commands(os.path.join(OUT, f))))
