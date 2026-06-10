# Idea for project

A miniatures wargame, played via UI and socket so that it can play human, ai, scripted, or any combination of that with any number of players.

A consistent arena - the field of play - is set up for each scenario, and simple but strict rules are followed for movement and fighting.

## Phases

Original Phase 1:
- scaffold enough of the app to display an arena with no active scenery (could have images, just no interaction), and two units, one for each of 2 players, with basic activation and movement.
- in the setup part, allow for different numbers of minis in the units, to exercise the different unit layout rules.
- units will activate in "rounds".  On the first round, a random player goes first, selecting a unit for activation.
- a successful "activation roll" means to roll 2 ten-sided dice, if either meets or exceeds the unit's "activation number", then that unit has successfully "activaated". Let's have one unit have an activation number of 5, and the other 4.
- on activation, the player can choose what the unit does from available actions (2 if successful, 1 simple if not), then play moves to the other player who selects a not-yet-activated unit to activate.
- unit should move or maneuver as selected.  Don't implement "shoot" in this phase.  If a maneuver is selected, allow a click to select the unit for the action (such as wheel) if needed. For api usage, each mini should have a key to identify action targets such as this.
- this should be client-server with json data coming back from the backend, driving the SVG on the frontend.

Current implemented scope:
- the app now supports saved army templates and rosters built from the imported unit catalog in `data/units.json`.
- games may start from one roster per player, or from manual setup units.
- the SVG arena supports multi-unit placement, activation, movement, pivot, about-face, skip, move into combat, combat resolution, morale, pending pushback/withdraw choices, win completion, action history, and rewind.
- wheel movement, shooting, special ability execution, sockets, and multiplayer auth are still deferred.
