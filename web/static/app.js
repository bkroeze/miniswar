function createMiniswarApp() {
  return {
    game: null,
    selectedUnit: "",
    selectedMini: "",
    messages: [],
    setup: {
      battlemapId: "old_road",
      player1: { base: "25x25", count: 12, units: [{ base: "25x25", count: 12 }] },
      player2: { base: "25x25", count: 10, units: [{ base: "25x25", count: 10 }] },
    },
    move: { direction: "forward", distanceMm: 50 },
    pivot: { facingDeg: 0 },

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
      const activation = this.game.currentActivation;
      if (activation) {
        return `Round ${this.game.round}, player ${activation.playerId}, ${activation.actionsRemaining} action(s) remaining`;
      }
      return `Round ${this.game.round}, player ${this.game.activePlayer} to activate`;
    },

    async setGame(game, options = {}) {
      this.game = game;
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

    renderArena() {
      this.renderTerrain();
      const root = this.$refs.units;
      if (!root) return;
      root.replaceChildren();
      const ns = "http://www.w3.org/2000/svg";
      for (const unit of this.game?.units || []) {
        const isActiveUnit = this.game?.currentActivation?.unitId === unit.id;
        const isSelectedForActivation = !this.game?.currentActivation && unit.id === this.selectedUnit && unit.playerId === this.game.activePlayer && !this.unitActivatedThisRound(unit.id);
        const pivotAxis = isActiveUnit ? this.pivotAxisKey() : "";
        const group = document.createElementNS(ns, "g");
        group.setAttribute("transform", `translate(${unit.x} ${unit.y}) rotate(${unit.facingDeg})`);
        group.addEventListener("click", () => {
          void this.selectUnit(unit);
        });

        for (const mini of unit.minis) {
          const miniGroup = document.createElementNS(ns, "g");
          miniGroup.setAttribute("transform", `translate(${mini.relX} ${mini.relY})`);
          miniGroup.addEventListener("click", (event) => {
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
