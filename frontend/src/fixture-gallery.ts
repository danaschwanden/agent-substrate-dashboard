import { LitElement, css, html } from "lit";
import "./components/actor-fleet-card.js";
import "./components/actor-lifecycle-card.js";
import "./components/actor-status-card.js";
import "./components/actor-timeline-card.js";
import "./components/control-api-card.js";
import "./components/operational-efficiency-card.js";
import "./components/operation-stream-card.js";
import "./components/transition-count-card.js";
import "./components/worker-map-card.js";
import "./components/worker-occupancy-card.js";
import type { ActorState, DashboardSnapshot } from "./types.js";

const fixtures = ["active", "crashed", "idle", "mixed", "partial-failure", "saturated", "stale", "truncated"];

class FixtureGallery extends LitElement {
  static properties = {
    fixture: { state: true },
    snapshot: { state: true },
    selectedActorNames: { state: true },
    error: { state: true },
  };

  static styles = css`
    :host {
      --sd-bg: #080e11; --sd-panel: #11191e; --sd-line: #2a383f; --sd-text: #e3eaed;
      --sd-muted: #89979e; --sd-green: #45d36b; --sd-blue: #66c3ff; --sd-pink: #ff64a5;
      --sd-yellow: #f2c14f; --sd-red: #ff5d73;
      display: block; min-height: 100dvh; color: var(--sd-text); background: var(--sd-bg);
      font-family: "SFMono-Regular", "Cascadia Code", "Roboto Mono", ui-monospace, monospace;
    }
    main { box-sizing: border-box; width: min(1440px, 100%); margin: auto; padding: 20px; }
    header { position: sticky; z-index: 2; top: 0; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 0; background: color-mix(in srgb, var(--sd-bg) 94%, transparent); backdrop-filter: blur(8px); }
    h1 { margin: 0; color: var(--sd-green); font-size: 18px; }
    p { margin: 4px 0 0; color: var(--sd-muted); font-size: 9px; }
    select { min-height: 34px; border: 1px solid var(--sd-line); border-radius: 5px; color: var(--sd-text); background: var(--sd-panel); font: inherit; font-size: 10px; }
    .gallery { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
    article { min-width: 0; }
    article > div { margin-bottom: 5px; color: var(--sd-muted); font-size: 8px; letter-spacing: .08em; text-transform: uppercase; }
    article > * { height: 190px; }
    article.compact > * { height: 105px; }
    article.wide { grid-column: 1 / -1; }
    .error { padding: 40px; color: var(--sd-red); text-align: center; }
    @media (max-width: 760px) { main { padding: 12px; } .gallery { grid-template-columns: 1fr; } article.wide { grid-column: auto; } }
  `;

  fixture = new URLSearchParams(window.location.search).get("fixture") ?? "mixed";
  snapshot?: DashboardSnapshot;
  selectedActorNames: string[] = [];
  error = "";

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
  }

  private async load(): Promise<void> {
    this.error = "";
    try {
      const response = await fetch(`/snapshots/${encodeURIComponent(this.fixture)}.json`);
      if (!response.ok) throw new Error(`Fixture ${this.fixture} is unavailable`);
      this.snapshot = await response.json() as DashboardSnapshot;
      this.selectedActorNames = [];
    } catch (error) {
      this.error = error instanceof Error ? error.message : "Fixture unavailable";
    }
  }

  private fixtureChanged(event: Event): void {
    this.fixture = (event.target as HTMLSelectElement).value;
    history.replaceState(null, "", `?fixture=${encodeURIComponent(this.fixture)}`);
    void this.load();
  }

  protected render() {
    const snapshot = this.snapshot;
    return html`<main>
      <header><div><h1>Dashboard component gallery</h1><p>Each example is an independent Lit component driven only by typed fixture properties.</p></div><label><select aria-label="Fixture" .value=${this.fixture} @change=${this.fixtureChanged}>${fixtures.map(name => html`<option value=${name}>${name}</option>`)}</select></label></header>
      ${this.error ? html`<div class="error" role="alert">${this.error}</div>` : snapshot ? this.renderGallery(snapshot) : html`<div class="error" role="status">Loading fixture…</div>`}
    </main>`;
  }

  private renderGallery(snapshot: DashboardSnapshot) {
    const activeStates = snapshot.actorSummary.states.filter(state => state.count > 0).map(state => state.state as ActorState);
    return html`<section class="gallery" aria-label="Component fixtures">
      <article class="compact"><div>&lt;actor-status-card&gt;</div><actor-status-card .summary=${snapshot.actorSummary}></actor-status-card></article>
      <article class="compact"><div>&lt;control-api-card&gt;</div><control-api-card .connection=${snapshot.connection} .frontendStale=${this.fixture === "stale"}></control-api-card></article>
      <article class="compact"><div>&lt;transition-count-card&gt;</div><transition-count-card .summary=${snapshot.transitionSummary}></transition-count-card></article>
      <article class="compact"><div>&lt;worker-occupancy-card&gt;</div><worker-occupancy-card .summary=${snapshot.workerSummary}></worker-occupancy-card></article>
      <article class="wide"><div>&lt;operational-efficiency-card&gt;</div><operational-efficiency-card .efficiency=${snapshot.efficiency} .workers=${snapshot.workers}></operational-efficiency-card></article>
      <article class="wide"><div>&lt;actor-lifecycle-card&gt;</div><actor-lifecycle-card .activeStates=${activeStates}></actor-lifecycle-card></article>
      <article><div>&lt;worker-map-card&gt;</div><worker-map-card .workers=${snapshot.workers} .truncation=${snapshot.truncation.workers}></worker-map-card></article>
      <article><div>&lt;actor-fleet-card&gt;</div><actor-fleet-card .actors=${snapshot.actors} .selectedActorNames=${this.selectedActorNames} .capabilities=${snapshot.capabilities} .truncation=${snapshot.truncation.actors} @actor-selection-change=${(event: CustomEvent<{ actorNames: string[] }>) => { this.selectedActorNames = event.detail.actorNames; }}></actor-fleet-card></article>
      <article><div>&lt;actor-timeline-card&gt;</div><actor-timeline-card .transitions=${snapshot.transitions}></actor-timeline-card></article>
      <article><div>&lt;operation-stream-card&gt;</div><operation-stream-card .operations=${snapshot.operations}></operation-stream-card></article>
    </section>`;
  }
}

customElements.define("fixture-gallery", FixtureGallery);
