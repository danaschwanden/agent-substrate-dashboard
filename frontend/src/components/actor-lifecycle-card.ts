import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { ActorState } from "../types.js";
import { stateClass, titleCase } from "../utils.js";

const lifecycle: ActorState[] = ["SUSPENDED", "RESUMING", "RUNNING", "SUSPENDING", "CRASHED", "REVERTING", "DELETING"];

export class ActorLifecycleCard extends LitElement {
  static properties = { activeStates: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-blue); padding: 13px; }
    .flow { display: flex; align-items: center; justify-content: center; gap: 7px; height: calc(100% - 18px); overflow-x: auto; }
    .node { min-width: 86px; padding: 9px 8px; border: 1px solid var(--sd-line); border-radius: 6px; background: #050a0d; text-align: center; opacity: .55; }
    .node.active { border-color: var(--state-color); box-shadow: 0 0 18px color-mix(in srgb, var(--state-color) 11%, transparent); opacity: 1; }
    .node strong { display: block; color: var(--sd-text); font-size: 10px; }
    .node span { display: block; margin-top: 4px; color: var(--state-color); font-size: 8px; text-transform: uppercase; }
    .arrow { color: var(--sd-muted); font-size: 13px; }
    .branch { display: grid; gap: 5px; }
    @container (max-width: 700px) { .flow { justify-content: flex-start; } }
  `];

  activeStates: ActorState[] = [];

  protected render() {
    return html`
      <section class="card" aria-labelledby="lifecycle-title">
        <h2 id="lifecycle-title" class="section-title">Actor lifecycle</h2>
        <div class="flow" aria-label="Observable actor lifecycle states">
          ${lifecycle.map((state, index) => html`
            ${index ? html`<span class="arrow" aria-hidden="true">→</span>` : ""}
            <div class="node ${stateClass(state)} ${this.activeStates.includes(state) ? "active" : ""}">
              <strong>${titleCase(state)}</strong><span>${this.activeStates.includes(state) ? "Observed" : "No actors"}</span>
            </div>
          `)}
        </div>
      </section>
    `;
  }
}

customElements.define("actor-lifecycle-card", ActorLifecycleCard);
