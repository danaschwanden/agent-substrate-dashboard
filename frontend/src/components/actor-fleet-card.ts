import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { Actor, TruncationDetail } from "../types.js";
import { stateClass, titleCase } from "../utils.js";

export interface ActorSelectionChangeDetail { actorNames: string[] }
export interface ActorOperationRequestDetail { operation: "resume" | "suspend"; actorNames: string[] }

export class ActorFleetCard extends LitElement {
  static properties = {
    actors: { attribute: false },
    selectedActorNames: { attribute: false },
    capabilities: { attribute: false },
    truncation: { attribute: false },
    busy: { type: Boolean },
  };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-pink); display: flex; flex-direction: column; padding: 13px; }
    .heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
    .actions { display: flex; gap: 6px; }
    button { min-height: 26px; border: 1px solid var(--sd-line); border-radius: 4px; color: var(--sd-text); background: #0a1115; font: inherit; font-size: 8px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
    button:not(:disabled):hover { border-color: var(--sd-blue); color: var(--sd-blue); cursor: pointer; }
    button:focus-visible, input:focus-visible { outline: 2px solid var(--sd-blue); outline-offset: 2px; }
    button:disabled { color: #647178; cursor: not-allowed; }
    .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; min-height: 0; margin-top: 10px; overflow: auto; }
    label { position: relative; display: grid; min-height: 42px; padding: 7px 8px 7px 25px; border: 1px solid var(--sd-line); border-left: 3px solid var(--state-color); border-radius: 5px; background: #050a0d; cursor: pointer; }
    label:has(input:checked) { border-color: var(--state-color); background: color-mix(in srgb, var(--state-color) 7%, #050a0d); }
    input { position: absolute; top: 9px; left: 7px; width: 11px; height: 11px; margin: 0; accent-color: var(--state-color); }
    strong { overflow: hidden; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
    .meta { overflow: hidden; color: var(--sd-muted); font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
    .notice { margin-top: 7px; color: var(--sd-yellow); font-size: 8px; }
    .policy { margin: 7px 0 0; color: var(--sd-yellow); font-size: 8px; }
    @container (max-width: 520px) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .heading { align-items: flex-start; } }
    @container (max-width: 330px) { .grid { grid-template-columns: 1fr; } .actions { display: none; } }
  `];

  actors: Actor[] = [];
  selectedActorNames: string[] = [];
  capabilities: string[] = [];
  truncation?: TruncationDetail;
  busy = false;

  protected render() {
    const selected = new Set(this.selectedActorNames);
    const operationMode = this.capabilities.includes("RESUME") || this.capabilities.includes("SUSPEND");
    const canResume = !this.busy && this.capabilities.includes("RESUME") && selected.size > 0;
    const canSuspend = !this.busy && this.capabilities.includes("SUSPEND") && selected.size > 0;
    return html`
      <section class="card" aria-labelledby="actor-fleet-title">
        <div class="heading">
          <h2 id="actor-fleet-title" class="section-title">Actor fleet</h2>
          <div class="actions" aria-label="Selected actor operations">
            <button ?disabled=${!canResume} aria-describedby="operation-policy" title=${canResume ? "Resume selected actors" : this.disabledReason("RESUME", selected.size)} @click=${() => this.request("resume")}>Resume ${selected.size || ""}</button>
            <button ?disabled=${!canSuspend} aria-describedby="operation-policy" title=${canSuspend ? "Suspend selected actors" : this.disabledReason("SUSPEND", selected.size)} @click=${() => this.request("suspend")}>Suspend ${selected.size || ""}</button>
          </div>
        </div>
        <div class="grid">
          ${this.actors.length ? this.actors.map(actor => html`
            <label class=${stateClass(actor.state)}>
              <input type="checkbox" .checked=${selected.has(actor.name)} aria-label=${`Select actor ${actor.name}`} @click=${(event: Event) => this.toggle(actor.name, (event.target as HTMLInputElement).checked)} />
              <strong>${actor.name}</strong>
              <span class="meta">${titleCase(actor.state)} · ${actor.workerName ?? actor.crashSummary ?? "unassigned"}</span>
            </label>
          `) : html`<div class="muted">No actors observed.</div>`}
        </div>
        ${!operationMode ? html`<p id="operation-policy" class="policy">Observe-only mode · lifecycle operations are disabled by backend policy.</p>` : this.busy ? html`<p id="operation-policy" class="policy">Lifecycle operation in progress…</p>` : html`<span id="operation-policy" hidden>Select actors before requesting an operation.</span>`}
        ${this.truncation?.truncated ? html`<div class="notice">Showing ${this.truncation.returned} of ${this.truncation.total} actors.</div>` : ""}
      </section>
    `;
  }

  private toggle(actorName: string, checked: boolean) {
    const names = new Set(this.selectedActorNames);
    checked ? names.add(actorName) : names.delete(actorName);
    this.selectedActorNames = [...names];
    this.dispatchEvent(new CustomEvent<ActorSelectionChangeDetail>("actor-selection-change", {
      detail: { actorNames: this.selectedActorNames }, bubbles: true, composed: true,
    }));
  }

  private request(operation: "resume" | "suspend") {
    this.dispatchEvent(new CustomEvent<ActorOperationRequestDetail>("actor-operation-request", {
      detail: { operation, actorNames: [...this.selectedActorNames] }, bubbles: true, composed: true,
    }));
  }

  private disabledReason(capability: string, selected: number) {
    if (this.busy) return "A lifecycle operation is already in progress";
    if (!this.capabilities.includes(capability)) return `${titleCase(capability)} is disabled by backend capability policy`;
    return selected ? "" : "Select one or more actors first";
  }
}

customElements.define("actor-fleet-card", ActorFleetCard);
