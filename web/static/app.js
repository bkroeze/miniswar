function createMiniswarApp() {
  return {
    game: null,
    selectedUnit: "",
    selectedMini: "",
    messages: [],
    placementPreview: null,
    setupSidebarCollapsed: false,
    newGameConfigOpen: false,
    loadGamesOpen: false,
    displayMode: "landing",
    gameReadOnly: false,
    savedGames: [],
    savedGamesLoading: false,
    armies: [],
    battlemaps: [],
    editorBattlemap: null,
    selectedTerrainId: "",
    armyDefaultsApplied: false,
    camera: { x: 0, y: 0, width: 760, height: 520 },
    cameraMapId: "",
    activeCommand: null,
    setup: {
      battlemapId: "old_road",
      player1ArmyId: "",
      player2ArmyId: "",
      player1: { base: "25x25", count: 12, units: [{ base: "25x25", count: 12 }] },
      player2: { base: "25x25", count: 10, units: [{ base: "25x25", count: 10 }] },
    },
    move: { direction: "forward", distanceMm: 100 },
    pivot: { facingDeg: 0 },

    isSetupPhase() {
      return this.game?.phase === "setup";
    },

    toggleSetupSidebar() {
      this.setupSidebarCollapsed = !this.setupSidebarCollapsed;
    },

    async initGame() {
      await Promise.all([this.loadBattlemaps(), this.loadArmies({ defaultSelections: true })]);
      const loadedFromURL = await this.loadGameFromLocation();
      if (loadedFromURL) return;
      this.ensureCameraForCurrentMap();
    },

    async initBattlemapsPage() {
      this.displayMode = "battlemaps";
      await this.loadBattlemaps();
      const first = this.battlemaps.find((battlemap) => battlemap.id === this.setup.battlemapId) || this.battlemaps[0];
      if (first) this.selectBattlemapForEdit(first.id);
      else this.newBattlemap();
    },

    showLanding() {
      return this.displayMode === "landing";
    },

    enterGameDisplay() {
      this.displayMode = "game";
    },

    async showNewGameConfig() {
      this.enterGameDisplay();
      await this.openNewGameConfig();
    },

    async showLoadGames() {
      this.enterGameDisplay();
      await this.openLoadGames();
    },

    async showBattlemapEditor() {
      window.location.href = "/battlemaps";
    },

    async openNewGameConfig() {
      await Promise.all([this.loadBattlemaps(), this.loadArmies()]);
      this.newGameConfigOpen = true;
      this.ensureCameraForCurrentMap();
    },

    closeNewGameConfig() {
      if (this.game) this.newGameConfigOpen = false;
    },

    setupPayload() {
      const parseBase = (value) => {
        const [baseWidthMm, baseDepthMm] = value.split("x").map((v) => Number(v));
        return { baseWidthMm, baseDepthMm };
      };
      return {
        battlemapId: this.setup.battlemapId,
        player1ArmyId: this.setup.player1ArmyId,
        player2ArmyId: this.setup.player2ArmyId,
        player1Units: this.setup.player1ArmyId ? [] : this.setup.player1.units.map((unit) => ({ ...parseBase(unit.base), count: unit.count })),
        player2Units: this.setup.player2ArmyId ? [] : this.setup.player2.units.map((unit) => ({ ...parseBase(unit.base), count: unit.count })),
      };
    },

    addSetupUnit(playerId) {
      const player = playerId === 1 ? this.setup.player1 : this.setup.player2;
      player.units.push({ base: player.base, count: player.count });
    },

    removeSetupUnit(playerId, index) {
      const units = playerId === 1 ? this.setup.player1.units : this.setup.player2.units;
      if (units.length > 1) units.splice(index, 1);
    },

    async createGame() {
      await Promise.all([this.loadBattlemaps(), this.loadArmies()]);
      const response = await this.api("/api/games", {
        method: "POST",
        body: JSON.stringify(this.setupPayload()),
      });
      if (response.ok) {
        this.newGameConfigOpen = false;
        await this.setGame(response.game, { resetSelection: true, readOnly: false });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async loadBattlemaps() {
      const response = await this.api("/api/battlemaps");
      if (response.ok) {
        this.battlemaps = response.battlemaps || [];
        if (!this.setup.battlemapId && this.battlemaps[0]) this.setup.battlemapId = this.battlemaps[0].id;
        if (this.setup.battlemapId && !this.battlemaps.some((battlemap) => battlemap.id === this.setup.battlemapId) && this.battlemaps[0]) {
          this.setup.battlemapId = this.battlemaps[0].id;
        }
        this.ensureCameraForCurrentMap();
      }
      if (!response.ok) {
        this.messages = [...(response.messages || []), ...(response.errors || [])];
      }
    },

    async loadArmies({ defaultSelections = false } = {}) {
      const response = await this.api("/api/armies");
      if (response.ok) {
        this.armies = response.armies || [];
        if (defaultSelections && !this.armyDefaultsApplied) {
          if (!this.setup.player1ArmyId && this.armies[0]) this.setup.player1ArmyId = this.armies[0].id;
          if (!this.setup.player2ArmyId && this.armies[1]) this.setup.player2ArmyId = this.armies[1].id;
          this.armyDefaultsApplied = true;
        }
      }
    },

    async openLoadGames() {
      this.loadGamesOpen = true;
      this.savedGamesLoading = true;
      const response = await this.api("/api/games");
      this.savedGamesLoading = false;
      if (response.ok) {
        this.savedGames = response.games || [];
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    closeLoadGames() {
      this.loadGamesOpen = false;
    },

    cloneBattlemap(battlemap) {
      return JSON.parse(JSON.stringify(battlemap));
    },

    selectBattlemapForEdit(id) {
      const battlemap = this.battlemaps.find((candidate) => candidate.id === id);
      if (!battlemap) return;
      this.editorBattlemap = this.cloneBattlemap(battlemap);
      this.editorBattlemap.terrains ||= [];
      this.selectedTerrainId = this.editorBattlemap.terrains[0]?.id || "";
    },

    newBattlemap() {
      this.editorBattlemap = {
        id: "",
        name: "New Battlemap",
        widthMm: 760,
        heightMm: 520,
        terrains: [],
      };
      this.selectedTerrainId = "";
    },

    editorViewBox() {
      const battlemap = this.editorBattlemap || { widthMm: 760, heightMm: 520 };
      return `0 0 ${battlemap.widthMm || 760} ${battlemap.heightMm || 520}`;
    },

    selectedTerrain() {
      return this.editorBattlemap?.terrains?.find((terrain) => terrain.id === this.selectedTerrainId) || null;
    },

    addEditorTerrain() {
      if (!this.editorBattlemap) return;
      this.editorBattlemap.terrains ||= [];
      const index = this.editorBattlemap.terrains.length + 1;
      const terrain = {
        id: `terrain-${Date.now()}`,
        type: "rough",
        label: "rough",
        shape: "rect",
        x: 25 * index,
        y: 25 * index,
        width: 100,
        height: 75,
      };
      this.editorBattlemap.terrains.push(terrain);
      this.selectedTerrainId = terrain.id;
    },

    deleteEditorTerrain() {
      if (!this.editorBattlemap || !this.selectedTerrainId) return;
      this.editorBattlemap.terrains = (this.editorBattlemap.terrains || []).filter((terrain) => terrain.id !== this.selectedTerrainId);
      this.selectedTerrainId = this.editorBattlemap.terrains[0]?.id || "";
    },

    async saveEditorBattlemap() {
      if (!this.editorBattlemap) return;
      const method = this.editorBattlemap.id ? "PATCH" : "POST";
      const path = this.editorBattlemap.id ? `/api/battlemaps/${this.editorBattlemap.id}` : "/api/battlemaps";
      const wasNew = !this.editorBattlemap.id;
      const response = await this.api(path, { method, body: JSON.stringify(this.editorBattlemap) });
      if (response.ok) {
        await this.loadBattlemaps();
        this.selectBattlemapForEdit(response.battlemap.id);
        if (wasNew) this.setup.battlemapId = response.battlemap.id;
        else this.setup.battlemapId ||= response.battlemap.id;
        this.renderArena();
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async deleteEditorBattlemap() {
      if (!this.editorBattlemap?.id) return;
      const response = await this.api(`/api/battlemaps/${this.editorBattlemap.id}`, { method: "DELETE" });
      if (response.ok) {
        await this.loadBattlemaps();
        const first = this.battlemaps[0];
        if (first) this.selectBattlemapForEdit(first.id);
        else this.newBattlemap();
        if (!this.battlemaps.some((battlemap) => battlemap.id === this.setup.battlemapId)) {
          this.setup.battlemapId = this.battlemaps[0]?.id || "";
        }
        this.renderArena();
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    gameRouteReference() {
      const match = window.location.pathname.match(/^\/games\/([^/]+)(?:\/steps\/(\d+))?\/?$/);
      if (!match) return null;
      return {
        id: decodeURIComponent(match[1]),
        step: match[2] === undefined ? null : Number(match[2]),
      };
    },

    async loadGameFromLocation() {
      const reference = this.gameRouteReference();
      if (!reference) return false;
      this.enterGameDisplay();
      const path =
        reference.step === null
          ? `/api/games/${encodeURIComponent(reference.id)}`
          : `/api/games/${encodeURIComponent(reference.id)}/steps/${reference.step}`;
      const response = await this.api(path);
      if (response.ok) {
        this.placementPreview = null;
        this.loadGamesOpen = false;
        await this.setGame(response.game, { resetSelection: true, readOnly: Boolean(response.readOnly) });
        this.messages = [`Loaded game ${reference.id} at step ${this.gameStep(response.game)}.`];
        if (this.isReadOnlyGame()) this.messages.push(this.readOnlyGameMessage());
        return true;
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
      return true;
    },

    gameStep(game = this.game) {
      return game?.actionHistory?.length || 0;
    },

    gameURL(game = this.game) {
      if (!game?.id) return "/";
      return `/games/${encodeURIComponent(game.id)}/steps/${this.gameStep(game)}`;
    },

    savedGameURL(saved) {
      return `/games/${encodeURIComponent(saved.id)}/steps/${saved.actionCount || 0}`;
    },

    syncGameURL(game = this.game) {
      if (!game?.id) return;
      const nextPath = this.gameURL(game);
      if (window.location.pathname === nextPath && !window.location.search && !window.location.hash) return;
      window.history.replaceState({ gameId: game.id, step: this.gameStep(game) }, "", nextPath);
    },

    isReadOnlyGame() {
      return Boolean(this.gameReadOnly);
    },

    readOnlyGameMessage() {
      return "Historical game steps are read-only. Open the latest step to continue playing.";
    },

    rejectReadOnlyAction() {
      if (!this.isReadOnlyGame()) return false;
      this.placementPreview = null;
      this.clearActiveCommand();
      this.messages = [this.readOnlyGameMessage()];
      this.renderArena();
      return true;
    },

    formatDate(value) {
      if (!value) return "";
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return value;
      return date.toLocaleString();
    },

    async activate() {
      if (this.rejectReadOnlyAction()) return;
      const unit = this.selectedActivatableUnit();
      if (!unit) return;
      this.clearActiveCommand();
      const response = await this.api(`/api/games/${this.game.id}/activate`, {
        method: "POST",
        body: JSON.stringify({ playerId: this.game.activePlayer, unitId: unit.id }),
      });
      if (response.ok) {
        await this.setGame(response.game);
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async confirmPlacement() {
      if (this.rejectReadOnlyAction()) return;
      const unit = this.currentPlacementUnit();
      if (!unit || !this.placementPreview) return;
      const response = await this.api(`/api/games/${this.game.id}/placements`, {
        method: "POST",
        body: JSON.stringify({
          playerId: unit.playerId,
          unitId: unit.id,
          x: this.placementPreview.officerX,
          y: this.placementPreview.officerY,
          facingDeg: this.placementPreview.facingDeg,
        }),
      });
      if (response.ok) {
        this.placementPreview = null;
        await this.setGame(response.game, { resetSelection: true });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
      if (!response.ok) {
        await this.renderArenaSoon();
      }
    },

    async takeAction(type) {
      if (this.rejectReadOnlyAction()) return;
      const unit = this.currentActivationUnit();
      if (!unit) return;
      const payload = { playerId: unit.playerId, unitId: unit.id, type };
      if (type === "move") Object.assign(payload, this.move);
      if (type === "pivot") Object.assign(payload, { ...this.pivot, anchorKey: this.pivotAxisKey() });
      this.clearActiveCommand();
      const response = await this.api(`/api/games/${this.game.id}/actions`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      if (response.ok) {
        await this.setGame(response.game, { resetPivotAxis: true });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async resolveCombatChoice(combatChoice) {
      if (this.rejectReadOnlyAction()) return;
      const choice = this.pendingCombatChoice();
      if (!choice) return;
      this.clearActiveCommand();
      const response = await this.api(`/api/games/${this.game.id}/actions`, {
        method: "POST",
        body: JSON.stringify({
          playerId: choice.winningPlayerId,
          unitId: choice.winningUnitId,
          type: "combat_pushback",
          combatChoice,
        }),
      });
      if (response.ok) {
        await this.setGame(response.game, { resetPivotAxis: true });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async rewind(actionIndex) {
      if (this.rejectReadOnlyAction()) return;
      const response = await this.api(`/api/games/${this.game.id}/rewind`, {
        method: "POST",
        body: JSON.stringify({ actionIndex }),
      });
      if (response.ok) {
        this.placementPreview = null;
        this.clearActiveCommand();
        await this.setGame(response.game, { resetSelection: true });
        this.renderArena();
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async api(path, options = {}) {
      const response = await fetch(path, {
        headers: { "Content-Type": "application/json" },
        ...options,
      });
      return await response.json();
    },

    activePlayerUnit() {
      return this.game?.units?.find((unit) => unit.playerId === this.game.activePlayer && this.unitHasActiveMinis(unit));
    },

    unitHasActiveMinis(unit) {
      return Boolean((unit?.minis || []).some((mini) => !mini.removed));
    },

    currentPlacementUnit() {
      if (!this.isSetupPhase()) return null;
      const active = (this.game?.units || []).find((unit) => unit.playerId === this.game.activePlayer && !unit.placed && this.unitHasActiveMinis(unit));
      if (active) return active;
      return (this.game?.units || []).find((unit) => !unit.placed && this.unitHasActiveMinis(unit)) || null;
    },

    selectedActivatableUnit() {
      const selected = this.game?.units?.find((unit) => unit.id === this.selectedUnit);
      if (selected && selected.playerId === this.game.activePlayer && !selected.broken && this.unitHasActiveMinis(selected) && !this.unitActivatedThisRound(selected.id)) {
        return selected;
      }
      return this.activatableUnits()[0];
    },

    activatableUnits() {
      return (this.game?.units || []).filter((unit) => unit.playerId === this.game.activePlayer && !unit.broken && this.unitHasActiveMinis(unit) && !this.unitActivatedThisRound(unit.id));
    },

    unitActivatedThisRound(unitId) {
      return Boolean(this.game?.actionHistory?.some((action) => action.round === this.game.round && action.type === "activate" && action.unitId === unitId));
    },

    currentActivationUnit() {
      const id = this.game?.currentActivation?.unitId;
      return this.game?.units?.find((unit) => unit.id === id);
    },

    selectedUnitObject() {
      if (!this.selectedUnit) return null;
      return this.game?.units?.find((unit) => unit.id === this.selectedUnit) || null;
    },

    detailUnit() {
      return this.currentActivationUnit() || this.selectedUnitObject();
    },

    detailInstructions() {
      if (!this.game) return "Start a new game to configure the battle.";
      if (this.isSetupPhase()) return "Place units:";
      return "Select a unit to inspect its stats.";
    },

    placementCounts() {
      return [1, 2].map((playerId) => {
        const units = (this.game?.units || []).filter((unit) => unit.playerId === playerId && this.unitHasActiveMinis(unit));
        return {
          playerId,
          total: units.length,
          remaining: units.filter((unit) => !unit.placed).length,
        };
      });
    },

    unitStatsEntries(unit) {
      const stats = unit?.stats || {};
      return [
        ["A", stats.a || unit?.activationNumber || "-"],
        ["M", stats.m || Math.round(this.movementLimitForUnit(unit) / 25) || "-"],
        ["F", stats.f || "-"],
        ["S", stats.s || "-"],
        ["D", stats.d || "-"],
        ["CD", stats.cd || "-"],
        ["H", stats.h || unit?.maxHealth || "-"],
      ].map(([label, value]) => ({ label, value }));
    },

    unitHealthLabel(unit) {
      if (!unit) return "-";
      const minis = unit.minis || [];
      if (minis.length) {
        const active = minis.filter((mini) => !mini.removed).length;
        const perMiniMax = unit.maxHealth || unit.stats?.h || unit.currentHealth || 1;
        const remainingHealth = minis.reduce((total, mini) => total + (mini.removed ? 0 : Math.max(0, mini.healthRemaining ?? perMiniMax)), 0);
        const maxHealth = minis.length * perMiniMax;
        if (perMiniMax > 1 || remainingHealth !== active) return `${remainingHealth}/${maxHealth} HP (${active}/${minis.length} minis)`;
        return `${active}/${minis.length} minis`;
      }
      if (unit.currentHealth || unit.maxHealth) return `${unit.currentHealth || 0}/${unit.maxHealth || unit.currentHealth || 0}`;
      return "-";
    },

    unitDetailStatus(unit) {
      if (!unit) return "-";
      const parts = [];
      if (unit.broken) parts.push("broken");
      if (unit.disordered) parts.push("disordered");
      if (this.isEngagedUnit(unit.id)) parts.push("engaged");
      if (this.game?.currentActivation?.unitId === unit.id) parts.push("active");
      if (!unit.placed) parts.push("unplaced");
      return parts.join(", ") || "ready";
    },

    unitDetailMeta(unit) {
      if (!unit) return "";
      const base = unit.base ? `${unit.base.widthMm}x${unit.base.depthMm}mm` : "unknown base";
      return `${unit.id} · ${base} · activation ${unit.activationNumber}`;
    },

    movementLimitForUnit(unit) {
      return unit?.movementLimitMm || ((unit?.stats?.m || 0) * 25) || 100;
    },

    resetMoveDistanceForUnit(unit) {
      this.move.distanceMm = this.movementLimitForUnit(unit);
    },

    pendingCombatChoice() {
      return this.game?.pendingCombatChoice || null;
    },

    clearActiveCommand() {
      this.activeCommand = null;
    },

    combatChoiceLabel(choice) {
      return {
        pushback_25: "Push 25mm",
        pushback_75: "Push 75mm",
        withdraw_25: "Withdraw 25mm",
        decline: "Decline",
      }[choice] || choice;
    },

    canActivate() {
      return this.game && !this.isReadOnlyGame() && !this.pendingCombatChoice() && !this.game.currentActivation && Boolean(this.selectedActivatableUnit());
    },

    canAct() {
      return Boolean(this.game?.currentActivation && !this.isReadOnlyGame() && !this.pendingCombatChoice());
    },

    isCommandActive(type, direction = "") {
      return this.activeCommand?.type === type && (!direction || this.activeCommand.direction === direction);
    },

    setActiveCommand(type, options = {}) {
      if (this.isCommandActive(type, options.direction || "")) {
        this.clearActiveCommand();
        this.renderArena();
        return;
      }
      this.activeCommand = { type, ...options };
      this.renderArena();
    },

    async skipActiveUnit() {
      await this.takeAction("skip");
    },

    selectedUnitLabel() {
      if (!this.game) return "";
      if (this.isSetupPhase()) {
        const unit = this.currentPlacementUnit();
        return unit ? `Player ${unit.playerId}: place ${unit.name}` : "Setup complete";
      }
      const unit = this.currentActivationUnit() || this.selectedActivatableUnit();
      return unit ? `${unit.name} (${unit.id})` : "No unit";
    },

    async selectUnit(unit) {
      if (!this.game) return;
      if (this.isSetupPhase()) {
        if (unit.placed) {
          this.selectedUnit = unit.id;
          this.selectedMini = "";
        }
        await this.renderArenaSoon();
        return;
      }
      if (this.game.currentActivation) {
        if (this.game.currentActivation.unitId === unit.id) {
          this.selectedUnit = unit.id;
        }
        if (this.game.currentActivation.unitId !== unit.id) {
          this.clearActiveCommand();
        }
        await this.renderArenaSoon();
        return;
      }
      if (unit.playerId === this.game.activePlayer && !unit.broken && this.unitHasActiveMinis(unit) && !this.unitActivatedThisRound(unit.id)) {
        this.selectedUnit = unit.id;
        this.selectedMini = "";
        this.clearActiveCommand();
      }
      await this.renderArenaSoon();
    },

    async selectMini(unit, mini) {
      if (!this.game) return;
      if (!this.game.currentActivation) {
        await this.selectUnit(unit);
        return;
      }
      if (this.game.currentActivation.unitId === unit.id) {
        this.selectedUnit = unit.id;
        this.selectedMini = mini.key;
      }
      await this.renderArenaSoon();
    },

    pivotAxisKey() {
      const unit = this.currentActivationUnit();
      if (!unit) return "";
      if (this.selectedMini && unit.minis.some((mini) => mini.key === this.selectedMini)) {
        return this.selectedMini;
      }
      return unit.minis.find((mini) => mini.isOfficer)?.key || "";
    },

    pivotAxisLabel() {
      const unit = this.currentActivationUnit();
      if (!unit) return "Pivot axis defaults to the officer after activation.";
      const axis = this.pivotAxisKey();
      const officer = unit.minis.find((mini) => mini.isOfficer)?.key;
      if (axis && axis !== officer) return `Pivot axis: ${axis}`;
      return `Pivot axis: officer ${officer || ""}`;
    },

    statusLine() {
      if (this.showLanding()) return "Choose how to begin";
      if (!this.game) return "Loading";
      if (this.isReadOnlyGame()) return `Read-only step ${this.gameStep()}`;
      if (this.game.phase === "complete") {
        return this.game.winnerPlayerId ? `Game complete: player ${this.game.winnerPlayerId} wins` : "Game complete: draw";
      }
      if (this.isSetupPhase()) {
        const unit = this.currentPlacementUnit();
        return unit ? `Setup: player ${unit.playerId} placing ${unit.name}` : "Setup";
      }
      const choice = this.pendingCombatChoice();
      if (choice) {
        return `Combat choice: player ${choice.winningPlayerId}, ${choice.winningUnitId}`;
      }
      const activation = this.game.currentActivation;
      if (activation) {
        return `Round ${this.game.round}, player ${activation.playerId}, ${activation.actionsRemaining} action(s) remaining`;
      }
      return `Round ${this.game.round}, player ${this.game.activePlayer} to activate`;
    },

    gameplayBanner() {
      if (!this.game) return "";
      if (this.isReadOnlyGame()) return `Read-only Historical Step ${this.gameStep()}`;
      if (this.game.phase === "complete") {
        return this.game.winnerPlayerId ? `Game Complete - Player ${this.game.winnerPlayerId} Wins` : "Game Complete - Draw";
      }
      const choice = this.pendingCombatChoice();
      if (choice) {
        return `Player ${choice.winningPlayerId} - Combat Choice`;
      }
      const activation = this.game.currentActivation;
      if (!activation) {
        return `Player ${this.game.activePlayer} - Activating`;
      }
      const total = this.activationActionTotal(activation);
      const current = Math.min(total, Math.max(1, total - activation.actionsRemaining + 1));
      const prefix = activation.success ? "Activation Succeeded" : "Simple Activation";
      return `${prefix} - Action ${current}/${total}`;
    },

    activationActionTotal(activation) {
      const activationRecord = [...(this.game?.actionHistory || [])]
        .reverse()
        .find((action) => action.type === "activate" && action.unitId === activation.unitId && action.round === this.game.round);
      if (!activation.success) return 1;
      if (activationRecord?.result?.wasDisordered && activationRecord?.result?.disorderCleared) return 1;
      return 2;
    },

    async setGame(game, options = {}) {
      const previousActivationUnitId = this.game?.currentActivation?.unitId || "";
      this.game = game;
      if (options.readOnly !== undefined) this.gameReadOnly = Boolean(options.readOnly);
      if (this.isReadOnlyGame()) {
        this.placementPreview = null;
        this.clearActiveCommand();
      }
      if (options.syncURL !== false) this.syncGameURL(game);
      this.ensureCameraForCurrentMap();
      const activationUnit = this.currentActivationUnit();
      if (activationUnit && activationUnit.id !== previousActivationUnitId) {
        this.resetMoveDistanceForUnit(activationUnit);
      }
      this.renderArena();
      if (this.isSetupPhase()) {
        if (options.resetSelection) this.selectedUnit = "";
        this.selectedMini = "";
        this.clearActiveCommand();
        await this.renderArenaSoon();
        return;
      }
      const selectedStillValid = this.game.units.some((unit) => unit.id === this.selectedUnit);
      const selectedCanActivate = this.selectedUnit && this.activatableUnits().some((unit) => unit.id === this.selectedUnit);
      if (options.resetSelection || !selectedStillValid || (!this.currentActivationUnit() && !selectedCanActivate)) {
        this.selectedUnit = this.currentActivationUnit()?.id || this.activatableUnits()[0]?.id || this.activePlayerUnit()?.id || "";
      }
      if (options.resetSelection || options.resetPivotAxis || !this.currentActivationUnit()) {
        this.selectedMini = "";
      }
      if (options.resetSelection || !this.currentActivationUnit() || this.pendingCombatChoice()) {
        this.clearActiveCommand();
      }
      await this.renderArenaSoon();
    },

    async renderArenaSoon() {
      await this.$nextTick();
      await new Promise((resolve) => requestAnimationFrame(resolve));
      this.renderArena();
      window.setTimeout(() => this.renderArena(), 0);
    },

    arenaPoint(event) {
      const eventElement = event.currentTarget || event.target;
      const svg = eventElement.ownerSVGElement || eventElement;
      const point = svg.createSVGPoint();
      point.x = event.clientX;
      point.y = event.clientY;
      const transformed = point.matrixTransform(svg.getScreenCTM().inverse());
      return { x: transformed.x, y: transformed.y };
    },

    activeBattlemap() {
      return this.game?.battlemap || this.battlemaps.find((battlemap) => battlemap.id === this.setup.battlemapId) || { id: "default", widthMm: 760, heightMm: 520, terrains: [] };
    },

    battlemapWidth() {
      return this.activeBattlemap().widthMm || 760;
    },

    battlemapHeight() {
      return this.activeBattlemap().heightMm || 520;
    },

    arenaViewBox() {
      this.ensureCameraForCurrentMap();
      return `${this.camera.x} ${this.camera.y} ${this.camera.width} ${this.camera.height}`;
    },

    ensureCameraForCurrentMap() {
      const battlemap = this.activeBattlemap();
      const mapKey = `${battlemap.id || "custom"}:${battlemap.widthMm || 760}x${battlemap.heightMm || 520}`;
      if (this.cameraMapId !== mapKey || !this.camera.width || !this.camera.height) {
        this.cameraMapId = mapKey;
        this.fitCameraToMap();
        return;
      }
      this.clampCamera();
    },

    fitCameraToMap() {
      this.camera = { x: 0, y: 0, width: this.battlemapWidth(), height: this.battlemapHeight() };
      this.clampCamera();
    },

    zoomCamera(factor) {
      this.ensureCameraForCurrentMap();
      const centerX = this.camera.x + this.camera.width / 2;
      const centerY = this.camera.y + this.camera.height / 2;
      const mapWidth = this.battlemapWidth();
      const mapHeight = this.battlemapHeight();
      let nextWidth = this.camera.width * factor;
      let nextHeight = this.camera.height * factor;
      const minSide = Math.min(nextWidth, nextHeight);
      if (minSide < 200) {
        const correction = 200 / minSide;
        nextWidth *= correction;
        nextHeight *= correction;
      }
      nextWidth = Math.min(nextWidth, mapWidth);
      nextHeight = Math.min(nextHeight, mapHeight);
      this.camera = { x: centerX - nextWidth / 2, y: centerY - nextHeight / 2, width: nextWidth, height: nextHeight };
      this.clampCamera();
      this.renderArena();
    },

    arenaWheel(event) {
      this.zoomCamera(event.deltaY < 0 ? 0.9 : 1.1);
    },

    panCamera(dx, dy) {
      this.ensureCameraForCurrentMap();
      this.camera.x += this.camera.width * dx;
      this.camera.y += this.camera.height * dy;
      this.clampCamera();
      this.renderArena();
    },

    clampCamera() {
      const mapWidth = this.battlemapWidth();
      const mapHeight = this.battlemapHeight();
      this.camera.width = Math.min(Math.max(this.camera.width, 1), mapWidth);
      this.camera.height = Math.min(Math.max(this.camera.height, 1), mapHeight);
      this.camera.x = Math.max(0, Math.min(this.camera.x, mapWidth - this.camera.width));
      this.camera.y = Math.max(0, Math.min(this.camera.y, mapHeight - this.camera.height));
    },

    async arenaClicked(event) {
      const clickedUnit = event.target.closest("[data-unit]");
      const clickedControl = event.target.closest("[data-arena-control]");
      if (clickedControl) return;
      if (this.isReadOnlyGame()) {
        if (!clickedUnit) this.rejectReadOnlyAction();
        return;
      }
      if (!this.isSetupPhase()) {
        if (clickedUnit) return;
        await this.handleActionMapClick(event);
        return;
      }
      if (clickedUnit && !clickedUnit.classList.contains("placement-preview")) return;
      const unit = this.currentPlacementUnit();
      if (!unit) return;
      const point = this.arenaPoint(event);
      const sameSpot =
        this.placementPreview?.unitId === unit.id &&
        Math.hypot(this.placementPreview.officerX - point.x, this.placementPreview.officerY - point.y) <= 24;
      const facingDeg = sameSpot ? (this.placementPreview.facingDeg + 15) % 360 : this.facingTowardArenaCenter(point.x, point.y);
      const officerX = sameSpot ? this.placementPreview.officerX : point.x;
      const officerY = sameSpot ? this.placementPreview.officerY : point.y;
      this.placementPreview = this.previewPlacement(unit, officerX, officerY, facingDeg);
      await this.renderArenaSoon();
    },

    async handleActionMapClick(event) {
      if (!this.canAct() || !this.activeCommand) return;
      const point = this.arenaPoint(event);
      if (this.activeCommand.type === "move") {
        await this.submitMoveAtPoint(point);
        return;
      }
      if (this.activeCommand.type === "pivot") {
        await this.submitPivotAtPoint(point);
      }
    },

    previewPlacement(unit, officerX, officerY, facingDeg) {
      const officer = unit.minis.find((mini) => mini.isOfficer) || unit.minis[0];
      const center = this.rotatePoint(officer.relX + officer.widthMm / 2, officer.relY + officer.depthMm / 2, facingDeg);
      return {
        unitId: unit.id,
        officerX,
        officerY,
        x: officerX - center.x,
        y: officerY - center.y,
        facingDeg,
      };
    },

    facingTowardArenaCenter(x, y) {
      const centerX = this.battlemapWidth() / 2;
      const centerY = this.battlemapHeight() / 2;
      const deg = (Math.atan2(centerX - x, -(centerY - y)) * 180) / Math.PI;
      return ((Math.round(deg / 45) * 45) % 360 + 360) % 360;
    },

    rotatePoint(x, y, deg) {
      const rad = (deg * Math.PI) / 180;
      return { x: x * Math.cos(rad) - y * Math.sin(rad), y: x * Math.sin(rad) + y * Math.cos(rad) };
    },

    unitWorldPoint(unit, x, y) {
      const point = this.rotatePoint(x, y, unit.facingDeg || 0);
      return { x: unit.x + point.x, y: unit.y + point.y };
    },

    miniWorldCenter(unit, mini) {
      return this.unitWorldPoint(unit, mini.relX + mini.widthMm / 2, mini.relY + mini.depthMm / 2);
    },

    unitWorldBounds(unit) {
      const activeMinis = (unit.minis || []).filter((mini) => !mini.removed);
      if (activeMinis.length === 0) return { minX: unit.x || 0, minY: unit.y || 0, maxX: unit.x || 0, maxY: unit.y || 0 };
      const points = [];
      for (const mini of activeMinis) {
        points.push(this.unitWorldPoint(unit, mini.relX, mini.relY));
        points.push(this.unitWorldPoint(unit, mini.relX + mini.widthMm, mini.relY));
        points.push(this.unitWorldPoint(unit, mini.relX, mini.relY + mini.depthMm));
        points.push(this.unitWorldPoint(unit, mini.relX + mini.widthMm, mini.relY + mini.depthMm));
      }
      return points.reduce(
        (bounds, point) => ({
          minX: Math.min(bounds.minX, point.x),
          minY: Math.min(bounds.minY, point.y),
          maxX: Math.max(bounds.maxX, point.x),
          maxY: Math.max(bounds.maxY, point.y),
        }),
        { minX: Infinity, minY: Infinity, maxX: -Infinity, maxY: -Infinity },
      );
    },

    unitWorldCenter(unit) {
      const bounds = this.unitWorldBounds(unit);
      return { x: (bounds.minX + bounds.maxX) / 2, y: (bounds.minY + bounds.maxY) / 2 };
    },

    controlSize() {
      const visibleSide = Math.min(this.camera.width || this.battlemapWidth(), this.camera.height || this.battlemapHeight());
      return Math.max(18, Math.min(42, visibleSide * 0.06));
    },

    moveVector(unit, direction) {
      const sign = direction === "backward" ? -1 : 1;
      const rad = ((unit.facingDeg || 0) * Math.PI) / 180;
      return { x: Math.sin(rad) * sign, y: -Math.cos(rad) * sign };
    },

    maxMoveDistance(unit, direction) {
      const activation = this.game?.currentActivation;
      let limit = this.movementLimitForUnit(unit);
      if (direction === "backward") limit /= 2;
      if ((activation?.movesTaken || 0) > 0) limit /= 2;
      return Math.max(1, limit);
    },

    async submitMoveAtPoint(point, requestedDirection = "") {
      const unit = this.currentActivationUnit();
      if (!unit || !this.canAct() || this.activeCommand?.type !== "move") return;
      const start = this.unitWorldCenter(unit);
      const forwardVector = this.moveVector(unit, "forward");
      const forwardProjection = (point.x - start.x) * forwardVector.x + (point.y - start.y) * forwardVector.y;
      const direction = requestedDirection || this.activeCommand?.direction || (forwardProjection < 0 ? "backward" : "forward");
      const vector = direction === "forward" ? forwardVector : this.moveVector(unit, direction);
      const maxDistance = this.maxMoveDistance(unit, direction);
      const projected = requestedDirection || this.activeCommand?.direction ? (point.x - start.x) * vector.x + (point.y - start.y) * vector.y : Math.abs(forwardProjection);
      this.move.direction = direction;
      this.move.distanceMm = Math.max(1, Math.min(maxDistance, Math.round(projected)));
      await this.takeAction("move");
    },

    async submitPivotAtPoint(point) {
      const unit = this.currentActivationUnit();
      if (!unit) return;
      const axis = unit.minis.find((mini) => mini.key === this.pivotAxisKey()) || unit.minis.find((mini) => mini.isOfficer) || unit.minis[0];
      if (!axis) return;
      const origin = this.miniWorldCenter(unit, axis);
      const dx = point.x - origin.x;
      const dy = point.y - origin.y;
      if (Math.hypot(dx, dy) < 1) return;
      const facingDeg = ((Math.round((Math.atan2(dx, -dy) * 180) / Math.PI) % 360) + 360) % 360;
      const backwards = (unit.facingDeg + 180) % 360;
      if (this.angleDifference(facingDeg, backwards) <= 10) {
        await this.takeAction("about_face");
        return;
      }
      this.pivot.facingDeg = facingDeg;
      await this.takeAction("pivot");
    },

    angleDifference(a, b) {
      const diff = Math.abs((((a - b) % 360) + 540) % 360 - 180);
      return diff;
    },

    svgElement(name, attrs = {}) {
      const element = document.createElementNS("http://www.w3.org/2000/svg", name);
      for (const [key, value] of Object.entries(attrs)) {
        element.setAttribute(key, value);
      }
      return element;
    },

    renderArena() {
      this.renderTerrain();
      const root = this.$refs.units;
      if (!root) return;
      root.replaceChildren();
      const ns = "http://www.w3.org/2000/svg";
      const pendingChoice = this.pendingCombatChoice();
      const engagedUnits = new Set();
      for (const engagement of this.game?.engagements || []) {
        if (!engagement.active) continue;
        engagedUnits.add(engagement.attackerUnitId);
        engagedUnits.add(engagement.defenderUnitId);
      }
      const units = (this.game?.units || []).filter((unit) => unit.placed && !unit.broken && this.unitHasActiveMinis(unit));
      if (this.isSetupPhase() && this.placementPreview) {
        const previewUnit = this.currentPlacementUnit();
        if (previewUnit) {
          units.push({ ...previewUnit, x: this.placementPreview.x, y: this.placementPreview.y, facingDeg: this.placementPreview.facingDeg, placed: true, preview: true });
        }
      }
      for (const unit of units) {
        const isActiveUnit = this.game?.currentActivation?.unitId === unit.id;
        const isSelectedUnit = unit.id === this.selectedUnit;
        const isSelectedForActivation = !this.game?.currentActivation && isSelectedUnit && unit.playerId === this.game.activePlayer && !this.unitActivatedThisRound(unit.id);
        const pivotAxis = isActiveUnit ? this.pivotAxisKey() : "";
        const isEngaged = engagedUnits.has(unit.id);
        const isWinner = pendingChoice?.winningUnitId === unit.id;
        const isLoser = pendingChoice?.losingUnitId === unit.id;
        const unitClasses = ["unit"];
        if (unit.preview) unitClasses.push("placement-preview");
        if (isEngaged) unitClasses.push("engaged");
        if (unit.disordered) unitClasses.push("disordered");
        if (isWinner) unitClasses.push("pending-winner");
        if (isLoser) unitClasses.push("pending-loser");
        const group = document.createElementNS(ns, "g");
        group.setAttribute("transform", `translate(${unit.x} ${unit.y}) rotate(${unit.facingDeg})`);
        group.setAttribute("data-unit", unit.id);
        group.setAttribute("class", unitClasses.join(" "));
        if (unit.preview) {
          group.setAttribute("pointer-events", "none");
        }
        group.addEventListener("click", () => {
          if (unit.preview) return;
          void this.selectUnit(unit);
        });

        for (const mini of unit.minis) {
          if (mini.removed) continue;
          const miniGroup = document.createElementNS(ns, "g");
          miniGroup.setAttribute("transform", `translate(${mini.relX} ${mini.relY})`);
          miniGroup.addEventListener("click", (event) => {
            if (unit.preview) return;
            event.stopPropagation();
            void this.selectMini(unit, mini);
          });

          const rect = document.createElementNS(ns, "rect");
          rect.setAttribute("width", mini.widthMm);
          rect.setAttribute("height", mini.depthMm);
          rect.setAttribute(
            "class",
            `mini p${unit.playerId}${isActiveUnit || isSelectedForActivation ? " active" : ""}${isSelectedUnit ? " selected-unit" : ""}${isActiveUnit && mini.key === pivotAxis ? " pivot-axis" : ""}`,
          );
          miniGroup.appendChild(rect);

          const text = document.createElementNS(ns, "text");
          text.setAttribute("x", mini.widthMm / 2);
          text.setAttribute("y", mini.depthMm / 2 + 4);
          text.setAttribute("text-anchor", "middle");
          text.setAttribute("class", "mini-text");
          text.textContent = mini.isOfficer ? "O" : mini.index;
          miniGroup.appendChild(text);
          group.appendChild(miniGroup);
        }
        const status = this.unitStatusText(unit, { isEngaged, isWinner, isLoser });
        if (status) {
          const bounds = this.localUnitBounds(unit);
          const badge = document.createElementNS(ns, "text");
          badge.setAttribute("x", bounds.minX);
          badge.setAttribute("y", bounds.minY - 6);
          badge.setAttribute("class", "unit-status-text");
          badge.textContent = status;
          group.appendChild(badge);
        }
        root.appendChild(group);
      }
      this.renderOverlays();
    },

    renderOverlays() {
      const root = this.$refs.overlays;
      if (!root) return;
      root.replaceChildren();
      if (!this.game || this.isReadOnlyGame() || this.isSetupPhase()) return;
      if (this.pendingCombatChoice()) {
        this.renderCombatChoiceOverlay(root);
        return;
      }
      if (this.canActivate()) {
        const unit = this.selectedActivatableUnit();
        if (unit?.placed) {
          const position = this.controlRowPosition(unit, 1);
          this.appendArenaControl(root, position.x, position.y, "+", unit.playerId, () => void this.activate(), "Activate unit");
        }
        return;
      }
      const unit = this.currentActivationUnit();
      if (!unit?.placed || !this.canAct()) return;
      const positions = this.controlRowPositions(unit, 3);
      this.appendArenaControl(root, positions[0].x, positions[0].y, ">", unit.playerId, () => this.setActiveCommand("move"), "Move", this.isCommandActive("move"));
      this.appendArenaControl(root, positions[1].x, positions[1].y, "*", unit.playerId, () => this.setActiveCommand("pivot"), "Pivot", this.isCommandActive("pivot"));
      this.appendArenaControl(root, positions[2].x, positions[2].y, "~", unit.playerId, () => void this.skipActiveUnit(), "Skip");
      if (this.activeCommand?.type === "move") {
        this.appendMoveArrow(root, unit, "forward");
        this.appendMoveArrow(root, unit, "backward");
      }
      if (this.activeCommand?.type === "pivot") {
        this.appendPivotStar(root, unit);
      }
    },

    controlRowPosition(unit, count) {
      return this.controlRowPositions(unit, count)[0];
    },

    controlRowPositions(unit, count) {
      const bounds = this.unitWorldBounds(unit);
      const size = this.controlSize();
      const gap = size * 0.22;
      const totalWidth = count * size + (count - 1) * gap;
      const startX = (bounds.minX + bounds.maxX) / 2 - totalWidth / 2 + size / 2;
      const y = bounds.maxY + size * 0.85;
      return Array.from({ length: count }, (_, index) => ({ x: startX + index * (size + gap), y }));
    },

    appendArenaControl(root, x, y, label, playerId, onClick, title, active = false) {
      const size = this.controlSize();
      const group = this.svgElement("g", { class: `arena-control p${playerId}${active ? " active" : ""}`, "data-arena-control": label, transform: `translate(${x} ${y})` });
      group.addEventListener("click", (event) => {
        event.stopPropagation();
        onClick();
      });
      group.appendChild(this.svgElement("title")).textContent = title;
      group.appendChild(this.svgElement("rect", { x: -size / 2, y: -size / 2, width: size, height: size, rx: size * 0.16, ry: size * 0.16 }));
      const text = this.svgElement("text", { x: 0, y: size * 0.13, "text-anchor": "middle", class: "arena-control-label" });
      text.textContent = label;
      group.appendChild(text);
      root.appendChild(group);
    },

    appendMoveArrow(root, unit, direction) {
      const start = this.unitWorldCenter(unit);
      const vector = this.moveVector(unit, direction);
      const distance = this.maxMoveDistance(unit, direction);
      const end = { x: start.x + vector.x * distance, y: start.y + vector.y * distance };
      const group = this.svgElement("g", { class: `move-arrow p${unit.playerId}`, "data-arena-control": `move-${direction}` });
      group.addEventListener("click", (event) => {
        event.stopPropagation();
        void this.submitMoveAtPoint(this.arenaPoint(event), direction);
      });
      group.appendChild(this.svgElement("title")).textContent = direction === "backward" ? "Move backward" : "Move forward";
      group.appendChild(this.svgElement("line", { class: "move-arrow-hit", x1: start.x, y1: start.y, x2: end.x, y2: end.y }));
      group.appendChild(this.svgElement("line", { x1: start.x, y1: start.y, x2: end.x, y2: end.y }));
      const head = this.controlSize() * 0.35;
      const angle = Math.atan2(vector.y, vector.x);
      const left = { x: end.x - Math.cos(angle - Math.PI / 6) * head, y: end.y - Math.sin(angle - Math.PI / 6) * head };
      const right = { x: end.x - Math.cos(angle + Math.PI / 6) * head, y: end.y - Math.sin(angle + Math.PI / 6) * head };
      group.appendChild(this.svgElement("path", { d: `M ${end.x} ${end.y} L ${left.x} ${left.y} L ${right.x} ${right.y} Z` }));
      root.appendChild(group);
    },

    appendPivotStar(root, unit) {
      const axis = unit.minis.find((mini) => mini.key === this.pivotAxisKey()) || unit.minis.find((mini) => mini.isOfficer) || unit.minis[0];
      if (!axis) return;
      const center = this.miniWorldCenter(unit, axis);
      const size = this.controlSize() * 0.38;
      const star = this.svgElement("text", { x: center.x, y: center.y + size * 0.32, "text-anchor": "middle", class: "pivot-star" });
      star.textContent = "*";
      root.appendChild(star);
    },

    renderCombatChoiceOverlay(root) {
      const choice = this.pendingCombatChoice();
      if (!choice) return;
      const unit = this.game?.units?.find((candidate) => candidate.id === choice.winningUnitId);
      if (!unit?.placed) return;
      const positions = this.controlRowPositions(unit, choice.choices?.length || 0);
      (choice.choices || []).forEach((combatChoice, index) => {
        this.appendArenaControl(root, positions[index].x, positions[index].y, this.combatChoiceGlyph(combatChoice), unit.playerId, () => void this.resolveCombatChoice(combatChoice), this.combatChoiceLabel(combatChoice));
      });
    },

    combatChoiceGlyph(choice) {
      return {
        pushback_25: "25",
        pushback_75: "75",
        withdraw_25: "<",
        decline: "x",
      }[choice] || "?";
    },

    isEngagedUnit(unitId) {
      return Boolean((this.game?.engagements || []).some((engagement) => engagement.active && (engagement.attackerUnitId === unitId || engagement.defenderUnitId === unitId)));
    },

    unitStatusText(unit, state = {}) {
      const parts = [];
      if (state.isEngaged ?? this.isEngagedUnit(unit.id)) parts.push("engaged");
      if (unit.disordered) parts.push("disordered");
      if (state.isWinner ?? this.pendingCombatChoice()?.winningUnitId === unit.id) parts.push("winner");
      if (state.isLoser ?? this.pendingCombatChoice()?.losingUnitId === unit.id) parts.push("pushed");
      return parts.join(" / ");
    },

    localUnitBounds(unit) {
      const activeMinis = (unit.minis || []).filter((mini) => !mini.removed);
      if (activeMinis.length === 0) return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
      return activeMinis.reduce(
        (bounds, mini) => ({
          minX: Math.min(bounds.minX, mini.relX),
          minY: Math.min(bounds.minY, mini.relY),
          maxX: Math.max(bounds.maxX, mini.relX + mini.widthMm),
          maxY: Math.max(bounds.maxY, mini.relY + mini.depthMm),
        }),
        { minX: Infinity, minY: Infinity, maxX: -Infinity, maxY: -Infinity },
      );
    },

    renderTerrain() {
      const root = this.$refs.terrain;
      if (!root) return;
      root.replaceChildren();
      const ns = "http://www.w3.org/2000/svg";
      for (const terrain of this.activeBattlemap().terrains || []) {
        if (terrain.shape !== "rect") continue;
        const rect = document.createElementNS(ns, "rect");
        rect.setAttribute("x", terrain.x);
        rect.setAttribute("y", terrain.y);
        rect.setAttribute("width", terrain.width);
        rect.setAttribute("height", terrain.height);
        rect.setAttribute("class", `terrain ${terrain.type}`);
        root.appendChild(rect);

        const label = document.createElementNS(ns, "text");
        label.setAttribute("x", terrain.x + terrain.width / 2);
        label.setAttribute("y", terrain.y + terrain.height / 2 + 4);
        label.setAttribute("text-anchor", "middle");
        label.setAttribute("class", `terrain-label ${terrain.type}`);
        label.textContent = terrain.label;
        root.appendChild(label);
      }
    },
  };
}

function createArmiesManager() {
  return {
    mode: "template",
    selectedId: "",
    selected: null,
    templates: [],
    armies: [],
    catalogUnits: [],
    filterOptions: { nations: [], terrains: [] },
    filters: { nation: "", terrain: "" },
    messages: [],

    async initArmies() {
      await Promise.all([this.loadFilters(), this.loadCatalog(), this.loadTemplates(), this.loadArmies()]);
      const template = this.templateFromURL();
      if (template) await this.selectTemplate(template.id);
      else if (this.templates[0]) await this.selectTemplate(this.templates[0].id);
      else if (this.armies[0]) await this.selectArmy(this.armies[0].id);
    },

    async api(path, options = {}) {
      const response = await fetch(path, { headers: { "Content-Type": "application/json" }, ...options });
      return await response.json();
    },

    async loadFilters() {
      const response = await this.api("/api/catalog/filters");
      if (response.ok) this.filterOptions = response.filters;
    },

    async loadCatalog() {
      const params = new URLSearchParams();
      if (this.filters.nation) params.set("nation", this.filters.nation);
      if (this.filters.terrain) params.set("terrain", this.filters.terrain);
      const response = await this.api(`/api/catalog/units?${params.toString()}`);
      if (response.ok) this.catalogUnits = response.units || [];
    },

    async loadTemplates() {
      const response = await this.api("/api/army-templates");
      if (response.ok) this.templates = response.templates || [];
    },

    async loadArmies() {
      const response = await this.api("/api/armies");
      if (response.ok) this.armies = response.armies || [];
    },

    async createTemplate() {
      const response = await this.api("/api/army-templates", { method: "POST", body: JSON.stringify({ name: "New Template", targetPoints: 1000 }) });
      await this.loadTemplates();
      if (response.ok) await this.selectTemplate(response.template.id);
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async createArmy() {
      const response = await this.api("/api/armies", { method: "POST", body: JSON.stringify({ name: "New Army", targetPoints: 1000 }) });
      await this.loadArmies();
      if (response.ok) await this.selectArmy(response.army.id);
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async createArmyFromSelected() {
      if (!this.selected || this.mode !== "template") return;
      const response = await this.api("/api/armies/from-template", {
        method: "POST",
        body: JSON.stringify({ templateId: this.selected.id, name: this.selected.name }),
      });
      await this.loadArmies();
      if (response.ok) await this.selectArmy(response.army.id);
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async selectTemplate(id) {
      const response = await this.api(`/api/army-templates/${id}`);
      if (response.ok) {
        this.mode = "template";
        this.selectedId = id;
        this.selected = response.template;
        this.updateTemplateURL(this.selected.name);
      }
    },

    async selectArmy(id) {
      const response = await this.api(`/api/armies/${id}`);
      if (response.ok) {
        this.mode = "army";
        this.selectedId = id;
        this.selected = response.army;
        this.clearTemplateURL();
      }
    },

    async saveSelectedMeta() {
      if (!this.selected) return;
      this.syncSelectedListSummary();
      const path = this.mode === "template" ? `/api/army-templates/${this.selected.id}` : `/api/armies/${this.selected.id}`;
      const response = await this.api(path, {
        method: "PATCH",
        body: JSON.stringify({ name: this.selected.name, targetPoints: this.selected.targetPoints }),
      });
      if (response.ok) this.selected = this.mode === "template" ? response.template : response.army;
      this.syncSelectedListSummary();
      if (this.mode === "template") this.updateTemplateURL(this.selected.name);
      await Promise.all([this.loadTemplates(), this.loadArmies()]);
    },

    async addUnit(unit) {
      if (!this.selected) {
        await this.createTemplate();
      }
      const path = this.mode === "template" ? `/api/army-templates/${this.selected.id}/units` : `/api/armies/${this.selected.id}/units`;
      const response = await this.api(path, {
        method: "POST",
        body: JSON.stringify({ catalogUnitId: unit.id, moniker: unit.unitName }),
      });
      if (response.ok) this.selected = this.mode === "template" ? response.template : response.army;
      if (this.mode === "template") this.updateTemplateURL(this.selected.name);
      await Promise.all([this.loadTemplates(), this.loadArmies()]);
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async saveLine(line) {
      this.syncSelectedListSummary();
      const path =
        this.mode === "template"
          ? `/api/army-templates/${this.selected.id}/units/${line.id}`
          : `/api/armies/${this.selected.id}/units/${line.id}`;
      const response = await this.api(path, {
        method: "PATCH",
        body: JSON.stringify({
          moniker: this.mode === "template" ? line.defaultMoniker : line.moniker,
          miniCount: line.miniCount,
          currentHealth: line.currentHealth,
        }),
      });
      if (response.ok) this.selected = this.mode === "template" ? response.template : response.army;
      this.syncSelectedListSummary();
      if (this.mode === "template") this.updateTemplateURL(this.selected.name);
      await Promise.all([this.loadTemplates(), this.loadArmies()]);
    },

    syncSelectedListSummary() {
      if (!this.selected) return;
      const list = this.mode === "template" ? this.templates : this.armies;
      const item = list.find((entry) => entry.id === this.selected.id);
      if (!item) return;
      item.name = this.selected.name;
      item.targetPoints = this.selected.targetPoints;
      item.totalPoints = this.entityPoints(this.selected);
    },

    templateFromURL() {
      const name = new URLSearchParams(window.location.search).get("template");
      if (!name) return null;
      const normalized = name.trim().toLocaleLowerCase();
      return this.templates.find((template) => template.name.trim().toLocaleLowerCase() === normalized) || null;
    },

    updateTemplateURL(name) {
      const url = new URL(window.location.href);
      url.searchParams.set("template", name);
      window.history.replaceState({}, "", url);
    },

    clearTemplateURL() {
      const url = new URL(window.location.href);
      url.searchParams.delete("template");
      window.history.replaceState({}, "", url);
    },

    async removeLine(line) {
      const path =
        this.mode === "template"
          ? `/api/army-templates/${this.selected.id}/units/${line.id}`
          : `/api/armies/${this.selected.id}/units/${line.id}`;
      const response = await this.api(path, { method: "DELETE" });
      if (response.ok) this.selected = this.mode === "template" ? response.template : response.army;
      if (this.mode === "template") this.updateTemplateURL(this.selected.name);
      await Promise.all([this.loadTemplates(), this.loadArmies()]);
    },

    pointsLine() {
      if (!this.selected) return "";
      const target = this.selected.targetPoints || 0;
      const total = this.entityPoints(this.selected);
      return total === target ? `${total}/${target} points` : `${total}/${target} points - total differs from target`;
    },

    linePoints(line) {
      return (line.catalogUnit?.pts || 0) * (line.miniCount || 0);
    },

    entityPoints(entity, kind = "") {
      if (!entity) return 0;
      if (entity.units) {
        return entity.units.reduce((total, line) => total + this.linePoints(line), 0);
      }
      if (this.selected && entity !== this.selected && entity.id === this.selected.id && (!kind || this.mode === kind)) {
        return this.entityPoints(this.selected);
      }
      return entity.totalPoints || 0;
    },

    statusLine() {
      if (!this.selected) return "Create a template or roster to begin.";
      return `${this.selected.name}: ${this.pointsLine()}`;
    },
  };
}

window.miniswar = createMiniswarApp;
window.armiesManager = createArmiesManager;
document.addEventListener("alpine:init", () => {
  window.Alpine.data("miniswar", createMiniswarApp);
  window.Alpine.data("armiesManager", createArmiesManager);
});
