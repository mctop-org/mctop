#!/usr/bin/env python3
"""Build mctop logo SVGs from real Anthrosevka Mono glyph outlines.

Brackets use the Regular weight, the wordmark uses Bold. Glyph contours are
emitted as <path> data so the assets carry no font dependency.
"""
import os
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen

HERE = os.path.dirname(os.path.abspath(__file__))
REG = TTFont(os.path.join(HERE, "fonts", "AnthrosevkaMono-Regular.ttf"))
BLD = TTFont(os.path.join(HERE, "fonts", "AnthrosevkaMono-Bold.ttf"))
UPM = REG["head"].unitsPerEm

IRIS_L, IRIS_D = "#907aa9", "#c4a7e7"
TXT_L, TXT_D = "#575279", "#e0def4"
BASE_D = "#191724"

def adv(f, ch="0"):
    return f["hmtx"][f.getBestCmap()[ord(ch)]][0]

def gpath(f, ch):
    gs = f.getGlyphSet()
    pen = SVGPathPen(gs)
    gs[f.getBestCmap()[ord(ch)]].draw(pen)
    return pen.getCommands()

def place(ch, font, x, color):
    return f'<path transform="translate({x:.1f},0)" d="{gpath(font, ch)}" fill="{color}"/>', x + adv(font, ch)

def wordmark(iris, txt, fname):
    gap = adv(REG) * 0.55
    paths, x = [], 0
    p, x = place("[", REG, x, iris); paths.append(p); x += gap
    for ch in "mctop":
        p, x = place(ch, BLD, x, txt); paths.append(p)
    x += gap
    p, x = place("]", REG, x, iris); paths.append(p)
    width = x
    top, bot = UPM * 0.80, -UPM * 0.20
    h = top - bot
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width:.0f} {h:.0f}" '
           f'width="{width:.0f}" height="{h:.0f}" role="img" aria-label="mctop">\n'
           f'<g transform="translate(0,{top:.0f}) scale(1,-1)">\n' + "\n".join(paths) + "\n</g></svg>\n")
    open(os.path.join(HERE, fname), "w").write(svg)
    return width, h

def icon(bg, iris, m, fname):
    inner_gap = adv(REG) * 0.16
    paths, x = [], 0
    p, x = place("[", REG, x, iris); paths.append(p); x += inner_gap
    p, x = place("m", BLD, x, m); paths.append(p); x += inner_gap
    p, x = place("]", REG, x, iris); paths.append(p)
    content_w = x
    side = UPM
    s = (side * 0.72) / content_w
    tx = (side - content_w * s) / 2
    baseline = side * 0.685
    r = side * 0.20
    bgrect = f'<rect width="{side:.0f}" height="{side:.0f}" rx="{r:.0f}" fill="{bg}"/>\n' if bg else ""
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {side:.0f} {side:.0f}" '
           f'width="{side:.0f}" height="{side:.0f}" role="img" aria-label="mctop">\n{bgrect}'
           f'<g transform="translate({tx:.1f},{baseline:.1f}) scale({s:.4f},{-s:.4f})">\n'
           + "\n".join(paths) + "\n</g></svg>\n")
    open(os.path.join(HERE, fname), "w").write(svg)

w, h = wordmark(IRIS_L, TXT_L, "wordmark-light.svg")
wordmark(IRIS_D, TXT_D, "wordmark-dark.svg")
icon(BASE_D, IRIS_D, TXT_D, "icon.svg")          # app/avatar tile, works on any bg
icon(None, IRIS_L, TXT_L, "icon-mark-light.svg") # bare mark, light
icon(None, IRIS_D, TXT_D, "icon-mark-dark.svg")  # bare mark, dark
print(f"wordmark {w:.0f}x{h:.0f}; icons built")
