# Remarks to various design discussions

The overall priority is to sort out the overall engine responsibilities and
the interplay of modules. It is not a priority to support every aspect of
high-quality PDF output immediately. We should be concerned with a solid
overall design, not with the implementation of every detail.

We are prepared to relax some properties of HTML/CSS layout standards in
a trade-off for sensible defaults for print.  Not everything which is
possible (or even sensible) in online-publishing may a priority (or even
sensible) in print.
Examples would be "Transforms, stacking levels, group opacity and blending",
or "Images, rich backgrounds, text decoration, outlines, rounded borders and
generated content".

Our immediate goal is to be able render a PDF for a no-nonsense 2-pager
without too-fancy paint. This spike will serve as a platform to 
extend scope and functionality.

## Semantic text

I understand that semantic text requirements are valid and necessary for a
high-quality PDF output. However, at the current stage we will postpone
this topic for later iterations. Having this correct will require keeping
more information along the pipeline, which will for now bloat the concept
and delude the clear understanding of responsibilities and separation of
concerns.

We will therefore stick with positioned glyph output from the layout phase,
without keeping the information of the original semantic text (for now).

## Interface to Folio

This needs special care. We do not want the idiosyncracies of Folio to 
bleed too much into our design decisions. There are other PDF solutions
out there, and we may even be willing in the future to create our own.
Folio will require us to serve its font requirements, because some of
Folio's internal functionality depends on it. That is fine. Examples
are font-subsetting, outline-rendering etc. But we will keep this local to
`docrender` and it strictly to be avoided that it bleeds into other
modules. We will probably load `opentype` fonts (sibling package) and
then additionally call Folio on the same font binary to have it construct
its private font data-structure. It will be the responsibility of `docrender`
to keep these 2 connected.

The user-chosen shaper will always be the authoritative source of glyph-selection
and positioning. We will therefore never rely on Folio's interpretation of
a glyphs standard advance-width, but rather on the advance info of the
typesetters glyphing phase.

## CSS paint traversal order

This is an algorithmic requirement, that we should ideally keep clearly
encapsulated. I am unsure right now in which module it should live.
Traversal needs special architectural care.
