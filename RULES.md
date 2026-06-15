# Rules

## Unit

A Unit is a group of one ore more "mini", also known as a "miniature".

## Base
Units have a "base size" in mm.

- 25x25mm
  - standard "human" size for minis
  - max unit size 20 minis
  - form in rows of 5, leftover in back rank, left flushed
- 25x50mm
  - cavalry, 4 legged beasts
  - max unit size 10 minis
  - form in rows of 5 as 25x25mm
- 50x50mm
  - large monsters such as trolls or ogres
  - max unit size 3
  - 1 row
- 50x100mm
  - largest monsters, such as dragons
  - always a unit of 1
- 100x50mm
  - artillery and other wide crews
  - always a unit of 1

All units designate one mini as the *officer*.  The officer must be placed in one of the two center positions of the first rank.  As minis are removed due to damage, the officer is never selected
until it is the last one remaining.  The officer should use a different marking or something to indicate which one it is.

## Facing

Units have facing.  This is a simple "X" quadrant with the center of the X being the middle point of the unit, disregarding any incomplete final rows if there are more than one row.  
The front facing quadrant is called "front", the others proceding clockwise are "right", "rear", and "left".

## Terrain

The arena is a battlemap measured in millimeters. Each battlemap defines its own width, height, and terrain zones.

- Rough terrain doubles movement cost only for the portion of a move where the unit overlaps it.
- Impassable terrain blocks placement, movement, pivot, about face, combat alignment, and pushback or withdraw movement.
- Path terrain is currently visual only.
- Passable obstacles do not block movement, but a unit moving into combat across or into contact with one counts as attacking an enemy behind fortifications.
- Units must remain within the active battlemap bounds. Pushback and withdraw movement stop at the edge of the map.

## Activating

During play, players alternate activating units.  Each unit is activated once before the next turn.

When activated, unless special rules apply, each unit gets 2 actions.  They can use these actions in any order.

- Move, up to the unit's movement limit in a straight line forward, or backwards up to half of the unit's movement.  If this is done a second time in one activation, the movement rate is halved.  A unit with an `M` stat moves `M * 25mm`; otherwise it uses the default `100mm`.
- Pivot, done by pivoting about the officer to any new direction
- About Face - which reorganizes the back line with a full line and moves the officer
- Skip - end the current activation's remaining actions

Wheel, shoot, and special abilities are planned rules, but are not currently available actions.

A unit that fails its activation roll may only take one *simple* action during its activation: move, pivot, about face, or skip.

## Move into Combat

When a unit moves into contact with any part of an enemy unit, it initiates a "move into combat".  When this occurs, the attacking unit is reoriented flush with the face of the unit they are attacking, centered on the officer of the attacking unit.

When units are "In combat" due to movement, or due to an activation of one of the enemy units already moved into combat with the unit, a "round of combat" is played.

### Round of Combat

- Determine Combat Dice
- Calculate Target Number
- Roll Combat Dice
- Determine Hits
- Apply Hits / Remove Casualties
- Morale Test
- Pushback

#### Determine Combat Dice

Each side has their own set of dice, calculated by taking the CD value from the unit, rolled X times, where X is:
- if the unit is facing forward and the enemy is contacted forward, then X = the number of units in the front rank.
- if the unit is being attacked by an enemy to any other face (left, right, rear), then X = the number of full ranks in the unit

The final CD is always adjusted to be at least 1, regardless of the prior calculation

#### Calculate Target Number

The target number is the number that needs to be met or exceeded on a D10 roll.

Calculation is: Defending unit's Defense stat (D) - Attacking Unit's Attack stat (A) + modifiers.

Modifiers are:
- ranks, only if fighting an enemy in the front facing: -(1 * number-of-full-ranks-in-unit -1)
- attacking left, right or rear: -1
- unit is defending its rear face: +1
- attacking unit is Disordered (a morale effect): +1
- unit is fighting from a lower elevation: +1 <-- this is a no-op for now, since we don't have elevations, but put in a hook for it with a comment
- unit moved into combat with an enemy behind fortifications: +1, detected when the move crosses or contacts a passable obstacle.

#### Roll Combat Dice

- Roll the sets of combat dice for each unit in the fight

#### Determine Hits

Count up the number of hits, which is:
- 1 for each roll that meets or exceeds the target number
- +1 if it meets or exceeds by 5
- +2 (total) if it meets or exceeds by 10

#### Apply hits / Remove Casualties

Each hit removes one point from the "H" value of the minis in a unit.  Most minis have only 1.  Track hits, starting at the highest numbered mini, and removing that mini when its H value hits 0.  A mini with at least 1 H left is fully-functional, but do track it for future calculations.

Always remove the officer last.

#### Morale test

If a unit loses figures, it must make a "Morale Test".

#### Pushback

The winner of the round of combat is presented with the option to pushback their opposing unit by 25mm or 75mm, or else to themselves withdraw by 25mm.  The pushback or withdraw must be on the same axis in the direction that the attacker was moving before.  The pushback automatically stops at obstacles or the edge of the table. They may also decline either option.

### Morale Tests

The player of a unit needing to make a morale test rolls 2D10 and compares the result to the target number.

Target Number starts at the "A" value for the unit, and is modified as follows:

- casualties: -1 per casualty the unit has suffered
- unit is Disordered: -1
- unit has less than one full rank: -1
- unit has 25x25 bases and has at least two full ranks: +1
- unit has 25x50 bases or 50x50 bases and at least one full rank: +1
- cause of morale test was a shooting attack: +1

Note that a unit-of-one (a champion or large monster) never has any modifiers and must take a morale test whenever it suffers any H damage.

If either die meets or exceeds the target number, the unit passes the test

If it fails, it becomes "Disordered".

If a Disordered unit fails its test, it becomes "Broken" and is removed from the battlefield, counting as a kill for the enemy.

A unit that is completely destroyed is removed and does not take a morale test.

A unit never takes more than one morale test in a turn. If multiple morale triggers would apply, roll only the first test and ignore later morale tests for that unit until the next turn.

A unit becoming "Broken" causes a morale test with no modifiers to friendly units within 8" of the unit.  This can cause a cascade among units owned by the same player.

When only one player has units left on the battlefield, that player wins. If no player has active units left, the game is a draw.

### Disordered Units

- Receive the penalties above
- Have a +1 on their activation values
- when they successfully activate, they remove the disordered status, but can only take a simple action that first round they have reactivated.
