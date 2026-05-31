function createMiniswarApp() {
  return {
    game: null,
    selectedUnit: "",
    selectedMini: "",
    messages: [],
    placementPreview: null,
    setup: {
      battlemapId: "old_road",
      player1: { base: "25x25", count: 12, units: [{ base: "25x25", count: 12 }] },
      player2: { base: "25x25", count: 10, units: [{ base: "25x25", count: 10 }] },
    },
    move: { direction: "forward", distanceMm: 50 },
    pivot: { facingDeg: 0 },

    isSetupPhase() {
      return this.game?.phase === "setup";
    },

    setupPayload() {
      const parseBase = (value) => {
        const [baseWidthMm, baseDepthMm] = value.split("x").map((v) => Number(v));
        return { baseWidthMm, baseDepthMm };
      };
      return {
        battlemapId: this.setup.battlemapId,
        player1Units: this.setup.player1.units.map((unit) => ({ ...parseBase(unit.base), count: unit.count })),
        player2Units: this.setup.player2.units.map((unit) => ({ ...parseBase(unit.base), count: unit.count })),
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
      const response = await this.api("/api/games", {
        method: "POST",
        body: JSON.stringify(this.setupPayload()),
      });
      if (response.ok) {
        await this.setGame(response.game, { resetSelection: true });
      }
      this.messages = [...(response.messages || []), ...(response.errors || [])];
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

    async rewind(actionIndex) {
      const response = await this.api(`/api/games/${this.game.id}/rewind`, {
        method: "POST",
        body: JSON.stringify({ actionIndex }),
      });
      if (response.ok) {
        await this.setGame(response.game, { resetSelection: true });
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
      if (selected && selected.playerId === this.game.activePlayer && !this.unitActivatedThisRound(selected.id)) {
        return selected;
      }
      return this.activatableUnits()[0];
    },

    activatableUnits() {
      return (this.game?.units || []).filter((unit) => unit.playerId === this.game.activePlayer && !this.unitActivatedThisRound(unit.id));
    },

    unitActivatedThisRound(unitId) {
      return Boolean(this.game?.actionHistory?.some((action) => action.round === this.game.round && action.type === "activate" && action.unitId === unitId));
    },

    currentActivationUnit() {
      const id = this.game?.currentActivation?.unitId;
      return this.game?.units?.find((unit) => unit.id === id);
    },

    canActivate() {
      return this.game && !this.game.currentActivation && Boolean(this.selectedActivatableUnit());
    },

    canAct() {
      return Boolean(this.game?.currentActivation);
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
      if (unit.playerId === this.game.activePlayer && !this.unitActivatedThisRound(unit.id)) {
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
      const activation = this.game.currentActivation;
      if (activation) {
        return `Round ${this.game.round}, player ${activation.playerId}, ${activation.actionsRemaining} action(s) remaining`;
      }
      return `Round ${this.game.round}, player ${this.game.activePlayer} to activate`;
    },

    async setGame(game, options = {}) {
      this.game = game;
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
      const units = (this.game?.units || []).filter((unit) => unit.placed);
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
        const group = document.createElementNS(ns, "g");
        group.setAttribute("transform", `translate(${unit.x} ${unit.y}) rotate(${unit.facingDeg})`);
        group.setAttribute("data-unit", unit.id);
        if (unit.preview) {
          group.setAttribute("class", "placement-preview");
          group.setAttribute("pointer-events", "none");
        }
        group.addEventListener("click", () => {
          if (unit.preview) return;
          void this.selectUnit(unit);
        });

        for (const mini of unit.minis) {
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
        root.appendChild(group);
      }
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

window.miniswar = createMiniswarApp;
document.addEventListener("alpine:init", () => {
  window.Alpine.data("miniswar", createMiniswarApp);
});
