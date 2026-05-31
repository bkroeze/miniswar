# Agents instructions

This is a Golang project, using sqlite for persistence of games and templUI for the browser UI.

React is overkill and is a bad match for SVG anyway, pick something else like Alpine or some SPA framework lighter than React for this, perhaps alpineJS, open to ideas here.

It is imperative that any action in game can also be taken with full, useful feedback for AI and other automation.

Full rewind is also important, when testing and running evals we may want to stage a fight at a given state or rewind the game in progress, 
or a completed game in a controlled manner for testing.

Simulation of the arena will be done graphically, using a grid notated as if it is millimeters.  This loosely maps to a "real world" size of
15mm = 1 meter.  Sizes of "minis", AKA "miniatures", scenery, and other elements will be given in mm.

The entire play area should be done in SVG, not a canvas.  For initial work, we will simply draw squares with different border colors and symbols for mini type, officer status, etc.

When minis move, they will move on this grid following the movement rules for minis and any special rules for the scenario, army, army races, personal power of the mini, or other environmental effect.

