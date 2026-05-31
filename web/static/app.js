function createMiniswarApp() {
  return {
    game: null,
    selectedUnit: "",
    selectedMini: "",
    messages: [],
    setup: {
      player1: { base: "25x25", count: 12 },
      player2: { base: "25x25", count: 10 },
    },
    move: { direction: "forward", distanceMm: 50 },
    pivot: { facingDeg: 0 },

    setupPayload() {
      const parseBase = (value) => {
        const [baseWidthMm, baseDepthMm] = value.split("x").map((v) => Number(v));
        return { baseWidthMm, baseDepthMm };
      };
      return {
        player1: { ...parseBase(this.setup.player1.base), count: this.setup.player1.count },
        player2: { ...parseBase(this.setup.player2.base), count: this.setup.player2.count },
      };
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
      const unit = this.activePlayerUnit();
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

    currentActivationUnit() {
      const id = this.game?.currentActivation?.unitId;
      return this.game?.units?.find((unit) => unit.id === id);
    },

    canActivate() {
      return this.game && !this.game.currentActivation;
    },

    canAct() {
      return Boolean(this.game?.currentActivation);
    },

    selectedUnitLabel() {
      if (!this.game) return "";
      const unit = this.currentActivationUnit() || this.activePlayerUnit();
      return unit ? `${unit.name} (${unit.id})` : "No unit";
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
      if (options.resetSelection || !this.selectedUnit) {
        this.selectedUnit = this.activePlayerUnit()?.id || "";
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
      const root = this.$refs.units;
      if (!root) return;
      root.replaceChildren();
      const ns = "http://www.w3.org/2000/svg";
      for (const unit of this.game?.units || []) {
        const isActiveUnit = this.game?.currentActivation?.unitId === unit.id;
        const pivotAxis = isActiveUnit ? this.pivotAxisKey() : "";
        const group = document.createElementNS(ns, "g");
        group.setAttribute("transform", `translate(${unit.x} ${unit.y}) rotate(${unit.facingDeg})`);
        group.addEventListener("click", () => {
          this.selectedUnit = unit.id;
        });

        for (const mini of unit.minis) {
          const miniGroup = document.createElementNS(ns, "g");
          miniGroup.setAttribute("transform", `translate(${mini.relX} ${mini.relY})`);
          miniGroup.addEventListener("click", (event) => {
            event.stopPropagation();
            this.selectedUnit = unit.id;
            if (!this.game?.currentActivation || this.game.currentActivation.unitId === unit.id) {
              this.selectedMini = mini.key;
            }
          });

          const rect = document.createElementNS(ns, "rect");
          rect.setAttribute("width", mini.widthMm);
          rect.setAttribute("height", mini.depthMm);
          rect.setAttribute(
            "class",
            `mini p${unit.playerId}${isActiveUnit ? " active" : ""}${isActiveUnit && mini.key === pivotAxis ? " pivot-axis" : ""}`,
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
  };
}

window.miniswar = createMiniswarApp;
document.addEventListener("alpine:init", () => {
  window.Alpine.data("miniswar", createMiniswarApp);
});
