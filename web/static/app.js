function createMiniswarApp() {
  return {
    game: null,
    selectedUnit: "",
    selectedMini: "",
    messages: [],
    placementPreview: null,
    setupSidebarCollapsed: false,
    loadGamesOpen: false,
    savedGames: [],
    savedGamesLoading: false,
    armies: [],
    setup: {
      battlemapId: "old_road",
      player1ArmyId: "",
      player2ArmyId: "",
      player1: { base: "25x25", count: 12, units: [{ base: "25x25", count: 12 }] },
      player2: { base: "25x25", count: 10, units: [{ base: "25x25", count: 10 }] },
    },
    move: { direction: "forward", distanceMm: 50 },
    pivot: { facingDeg: 0 },

    isSetupPhase() {
      return this.game?.phase === "setup";
    },

    toggleSetupSidebar() {
      this.setupSidebarCollapsed = !this.setupSidebarCollapsed;
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
      await this.loadArmies();
      const response = await this.api("/api/games", {
        method: "POST",
        body: JSON.stringify(this.setupPayload()),
      });
      if (response.ok) {
        await this.setGame(response.game, { resetSelection: true });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    async loadArmies() {
      const response = await this.api("/api/armies");
      if (response.ok) {
        this.armies = response.armies || [];
        if (!this.setup.player1ArmyId && this.armies[0]) this.setup.player1ArmyId = this.armies[0].id;
        if (!this.setup.player2ArmyId && this.armies[1]) this.setup.player2ArmyId = this.armies[1].id;
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

    async loadGame(gameId) {
      const response = await this.api(`/api/games/${gameId}`);
      if (response.ok) {
        this.placementPreview = null;
        this.loadGamesOpen = false;
        await this.setGame(response.game, { resetSelection: true });
        this.messages = [`Loaded game ${gameId}.`];
        return;
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
    },

    formatDate(value) {
      if (!value) return "";
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return value;
      return date.toLocaleString();
    },

    async activate() {
      const unit = this.selectedActivatableUnit();
      if (!unit) return;
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
      const unit = this.currentActivationUnit();
      if (!unit) return;
      const payload = { playerId: unit.playerId, unitId: unit.id, type };
      if (type === "move") Object.assign(payload, this.move);
      if (type === "pivot") Object.assign(payload, { ...this.pivot, anchorKey: this.pivotAxisKey() });
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
      const choice = this.pendingCombatChoice();
      if (!choice) return;
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
      const response = await this.api(`/api/games/${this.game.id}/rewind`, {
        method: "POST",
        body: JSON.stringify({ actionIndex }),
      });
      if (response.ok) {
        this.placementPreview = null;
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
      return this.game?.units?.find((unit) => unit.playerId === this.game.activePlayer);
    },

    currentPlacementUnit() {
      if (!this.isSetupPhase()) return null;
      const active = (this.game?.units || []).find((unit) => unit.playerId === this.game.activePlayer && !unit.placed);
      if (active) return active;
      return (this.game?.units || []).find((unit) => !unit.placed) || null;
    },

    selectedActivatableUnit() {
      const selected = this.game?.units?.find((unit) => unit.id === this.selectedUnit);
      if (selected && selected.playerId === this.game.activePlayer && !selected.broken && !this.unitActivatedThisRound(selected.id)) {
        return selected;
      }
      return this.activatableUnits()[0];
    },

    activatableUnits() {
      return (this.game?.units || []).filter((unit) => unit.playerId === this.game.activePlayer && !unit.broken && !this.unitActivatedThisRound(unit.id));
    },

    unitActivatedThisRound(unitId) {
      return Boolean(this.game?.actionHistory?.some((action) => action.round === this.game.round && action.type === "activate" && action.unitId === unitId));
    },

    currentActivationUnit() {
      const id = this.game?.currentActivation?.unitId;
      return this.game?.units?.find((unit) => unit.id === id);
    },

    pendingCombatChoice() {
      return this.game?.pendingCombatChoice || null;
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
      return this.game && !this.pendingCombatChoice() && !this.game.currentActivation && Boolean(this.selectedActivatableUnit());
    },

    canAct() {
      return Boolean(this.game?.currentActivation && !this.pendingCombatChoice());
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
      if (this.game.currentActivation) {
        if (this.game.currentActivation.unitId === unit.id) {
          this.selectedUnit = unit.id;
        }
        await this.renderArenaSoon();
        return;
      }
      if (unit.playerId === this.game.activePlayer && !unit.broken && !this.unitActivatedThisRound(unit.id)) {
        this.selectedUnit = unit.id;
        this.selectedMini = "";
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
      if (!this.game) return "Loading";
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

    async setGame(game, options = {}) {
      this.game = game;
      this.renderArena();
      if (this.isSetupPhase()) {
        this.selectedUnit = this.currentPlacementUnit()?.id || "";
        this.selectedMini = "";
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
      await this.renderArenaSoon();
    },

    async renderArenaSoon() {
      await this.$nextTick();
      await new Promise((resolve) => requestAnimationFrame(resolve));
      this.renderArena();
      window.setTimeout(() => this.renderArena(), 0);
    },

    arenaPoint(event) {
      const svg = event.currentTarget;
      const point = svg.createSVGPoint();
      point.x = event.clientX;
      point.y = event.clientY;
      const transformed = point.matrixTransform(svg.getScreenCTM().inverse());
      return { x: transformed.x, y: transformed.y };
    },

    async arenaClicked(event) {
      if (!this.isSetupPhase()) return;
      const clickedUnit = event.target.closest("[data-unit]");
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
      const deg = (Math.atan2(380 - x, -(260 - y)) * 180) / Math.PI;
      return ((Math.round(deg / 45) * 45) % 360 + 360) % 360;
    },

    rotatePoint(x, y, deg) {
      const rad = (deg * Math.PI) / 180;
      return { x: x * Math.cos(rad) - y * Math.sin(rad), y: x * Math.sin(rad) + y * Math.cos(rad) };
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
      const units = (this.game?.units || []).filter((unit) => unit.placed && !unit.broken);
      if (this.isSetupPhase() && this.placementPreview) {
        const previewUnit = this.currentPlacementUnit();
        if (previewUnit) {
          units.push({ ...previewUnit, x: this.placementPreview.x, y: this.placementPreview.y, facingDeg: this.placementPreview.facingDeg, placed: true, preview: true });
        }
      }
      for (const unit of units) {
        const isActiveUnit = this.game?.currentActivation?.unitId === unit.id;
        const isSelectedForActivation = !this.game?.currentActivation && unit.id === this.selectedUnit && unit.playerId === this.game.activePlayer && !this.unitActivatedThisRound(unit.id);
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
            `mini p${unit.playerId}${isActiveUnit || isSelectedForActivation ? " active" : ""}${isSelectedForActivation ? " selected-unit" : ""}${isActiveUnit && mini.key === pivotAxis ? " pivot-axis" : ""}`,
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
      for (const terrain of this.game?.battlemap?.terrains || []) {
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
