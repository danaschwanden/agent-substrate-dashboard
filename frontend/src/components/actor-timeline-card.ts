import { LitElement, css, html } from "lit";
import { cardStyles } from "../styles/card-styles.js";
import type { ObservedTransition } from "../types.js";
import { formatClock, stateClass, titleCase } from "../utils.js";

export class ActorTimelineCard extends LitElement {
  static properties = { transitions: { attribute: false } };
  static styles = [cardStyles, css`
    .card { --accent: var(--sd-blue); display: flex; flex-direction: column; padding: 13px; }
    .list { min-height: 0; margin-top: 8px; overflow: auto; }
    .event { display: grid; grid-template-columns: 56px minmax(90px, .8fr) minmax(150px, 1.4fr); gap: 8px; align-items: center; padding: 6px 3px; border-bottom: 1px dashed #263137; font-size: 8px; }
    time { color: var(--sd-muted); }
    strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .change { color: var(--sd-muted); text-align: right; }
    .arrow { margin: 0 5px; color: var(--sd-muted); }
    @container (max-width: 430px) { .event { grid-template-columns: 50px 1fr; } .change { grid-column: 2; text-align: left; } }
  `];

  transitions: ObservedTransition[] = [];

  protected render() {
    return html`
      <section class="card" aria-labelledby="timeline-title">
        <h2 id="timeline-title" class="section-title">Actor task timeline</h2>
        <div class="list">
          ${this.transitions.length ? this.transitions.map(transition => html`
            <div class="event">
              <time datetime=${transition.observedAt}>${formatClock(transition.observedAt)}</time>
              <strong>${transition.actorName}</strong>
              <span class="change"><span class=${stateClass(transition.previousState)}>${titleCase(transition.previousState)}</span><span class="arrow">→</span><span class=${stateClass(transition.currentState)}>${titleCase(transition.currentState)}</span></span>
            </div>
          `) : html`<div class="muted">No lifecycle transitions observed since backend start.</div>`}
        </div>
      </section>
    `;
  }
}

customElements.define("actor-timeline-card", ActorTimelineCard);
