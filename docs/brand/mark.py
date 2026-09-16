"""Variants on the existing bowtie mark.

The current logo, traced off app/build/appicon.png, is two congruent triangles
related by a 180 degree rotation about the centre: each has a full-width
horizontal base and an apex that reaches past the centre on the far side, so
they overlap into a sheared hourglass. Everything here keeps that construction
or deliberately states how it departs from it.

The brief comes from the measured landscape: abstract marks in this category
run 3 to 23 path commands. Every variant is built from straight segments and
kept inside that band, so the counts are printed at the end.
"""
import math, os, re

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "drafts-mark")
os.makedirs(OUT, exist_ok=True)

BRAND = "#AC2D31"
PAPER = "#FAFAFA"
DEEP = "#7C1E22"

# Traced geometry, normalised so the mark fills 10..90 in a 100 box.
TOP = [(10, 10), (90, 10), (38.6, 61.6)]
BOT = [(90, 90), (10, 90), (61.4, 38.4)]


def poly(points, close=True):
    d = "M%.1f,%.1f" % points[0]
    for p in points[1:]:
        d += "L%.1f,%.1f" % p
    return d + ("Z" if close else "")


def rot180(p, cx=50, cy=50):
    return (2 * cx - p[0], 2 * cy - p[1])


def scale(points, k, cx=50, cy=50):
    return [(cx + (x - cx) * k, cy + (y - cy) * k) for x, y in points]


def svg(name, body, w=100, defs=""):
    doc = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">\n%s%s\n</svg>\n'
           % (w, w, defs, body))
    assert (len(re.findall(r'<(?:path|circle|rect|stop|polygon)\b', doc))
            == len(re.findall(r'/>', doc))), name
    open(os.path.join(OUT, name + ".svg"), "w").write(doc)


def fill(d, colour=BRAND, extra=""):
    return '<path d="%s" fill="%s"%s/>' % (d, colour, extra)


def rounded(points, r, colour=BRAND, span=40.0):
    """Rounded corners without arc commands: a round-joined stroke of the
    body colour. The stroke grows the shape by r/2, so pre-scale about the
    mark centre to land back on the original extent. A true edge inset would
    be exact but inverts on an apex this acute."""
    return ('<path d="%s" fill="%s" stroke="%s" stroke-width="%.1f" '
            'stroke-linejoin="round"/>'
            % (poly(scale(points, 1 - r / (2 * span))), colour, colour, r))


SQ = '<rect x="2" y="2" width="96" height="96" rx="24" fill="%s"/>\n'

# 01 -- the tidy-up: same construction, exact rotational symmetry, one flat fill.
svg("clean", fill(poly(TOP) + poly(BOT)))

# 02 -- softened corners, the register Linear and Raycast work in.
svg("round", rounded(TOP, 10) + rounded(BOT, 10))

# 03 -- a straight diagonal cut through the waist. Negative space is the single
#       most common move in current dev-tool marks.
svg("cut", '<g mask="url(#m)">%s</g>' % fill(poly(TOP) + poly(BOT)),
    defs='<mask id="m"><rect width="100" height="100" fill="#fff"/>'
         '<rect x="-30" y="43" width="160" height="14" fill="#000" '
         'transform="rotate(-24,50,50)"/></mask>\n')

# 04 -- symmetric hourglass with a real waist. SELECT is a filter, and this is
#       the shape of one; it trades the skew for legible meaning.
FUNNEL_T = [(12, 12), (88, 12), (56, 50), (44, 50)]
svg("funnel", fill(poly(FUNNEL_T) + poly([rot180(p) for p in FUNNEL_T])))

# 05 -- two chevrons: the same diagonals, read as flow rather than as an object.
def chev(y, h=24, w=33, t=15):
    """Stroked, not outlined: a hand-built chevron outline doubles back on
    itself at the tips and the fill rule eats the result."""
    return ('<path d="M%.1f,%.1f L50,%.1f L%.1f,%.1f" fill="none" stroke="%s" '
            'stroke-width="%.1f" stroke-linecap="round" stroke-linejoin="round"/>'
            % (50 - w, y, y + h, 50 + w, y, BRAND, t))

svg("chevron", chev(22) + chev(50))

# 06 -- three narrowing bars: a result set being filtered. Furthest from the
#       current mark, closest to the category's current house style.
def bars():
    out = []
    for i, (wdt, y) in enumerate(((78, 16), (56, 42), (34, 68))):
        out.append('<rect x="%.1f" y="%d" width="%.1f" height="16" rx="8" fill="%s"/>'
                   % (50 - wdt / 2, y, wdt, BRAND))
    return "\n".join(out)

svg("rows", bars())

# 07 -- the tidy-up with the brand hue given depth.
svg("grad", fill(poly(TOP) + poly(BOT), "url(#g)"),
    defs='<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">'
         '<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>'
         '</linearGradient></defs>\n' % (BRAND, DEEP))

# 08 / 09 -- grounds, for the icon and for circular crops.
svg("tile", rounded(scale(TOP, 0.66), 7, PAPER, span=26)
    + rounded(scale(BOT, 0.66), 7, PAPER, span=26), defs=SQ % BRAND)
svg("badge", rounded(scale(TOP, 0.58), 6, PAPER, span=23)
    + rounded(scale(BOT, 0.58), 6, PAPER, span=23),
    defs='<circle cx="50" cy="50" r="48" fill="%s"/>\n' % BRAND)


def commands(path):
    s = open(path).read()
    return (sum(len(re.findall(r'[MmLlHhVvCcSsQqTtAaZz]', d))
                for d in re.findall(r'\sd="([^"]*)"', s))
            + len(re.findall(r'<(?:rect|circle)\b', s)))


for f in sorted(os.listdir(OUT)):
    print("%-10s %3d commands" % (f[:-4], commands(os.path.join(OUT, f))))
