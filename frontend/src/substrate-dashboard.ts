import { LitElement, css, html } from "lit";
import { DashboardApiClient } from "./api-client.js";
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
import type { ActorOperationRequestDetail, ActorSelectionChangeDetail } from "./components/actor-fleet-card.js";
import type { ActorState, DashboardSnapshot } from "./types.js";
import { formatClock } from "./utils.js";

const supportedApiVersion = "v1";

export class SubstrateDashboard extends LitElement {
  static properties = {
    fixture: { type: String },
    pollInterval: { type: Number, attribute: "poll-interval" },
    snapshot: { attribute: false },
    message: { state: true },
    frontendStale: { state: true },
    incompatibleVersion: { state: true },
    selectedActorNames: { state: true },
    operationMessage: { state: true },
    operationPending: { state: true },
  };

  static styles = css`
    :host {
      --sd-bg: #080e11;
      --sd-panel: #11191e;
      --sd-line: #2a383f;
      --sd-text: #e3eaed;
      --sd-muted: #89979e;
      --sd-green: #45d36b;
      --sd-blue: #66c3ff;
      --sd-pink: #ff64a5;
      --sd-yellow: #f2c14f;
      --sd-red: #ff5d73;
      display: block;
      min-height: 100dvh;
      color: var(--sd-text);
      background: radial-gradient(circle at 50% -20%, #15252a 0, var(--sd-bg) 43%);
      font-family: "SFMono-Regular", "Cascadia Code", "Roboto Mono", ui-monospace, monospace;
    }
    .shell {
      box-sizing: border-box;
      display: grid;
      grid-template-rows: 44px 92px 116px 100px minmax(190px, 1fr) 170px;
      gap: 10px;
      height: 100dvh;
      min-height: 760px;
      padding: 12px;
      overflow: hidden;
    }
    header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid color-mix(in srgb, var(--sd-green) 75%, transparent); }
    h1 { margin: 0; color: var(--sd-green); font-size: clamp(16px, 2vw, 21px); letter-spacing: .02em; }
    h1 span { margin-left: 8px; color: var(--sd-muted); font-size: 10px; font-weight: 500; letter-spacing: .06em; }
    .scope, time, .operation-message { color: var(--sd-muted); font-size: 9px; letter-spacing: .06em; text-transform: uppercase; }
    .operation-message { color: var(--sd-yellow); }
    .clock::before { margin-right: 7px; color: var(--sd-muted); content: "●"; font-size: 7px; }
    .summary-grid, .detail-grid, .history-grid { display: grid; gap: 10px; min-height: 0; }
    .summary-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .detail-grid, .history-grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
    .summary-grid > *, .detail-grid > *, .history-grid > *, operational-efficiency-card, actor-lifecycle-card { min-width: 0; min-height: 0; }
    .status { display: grid; min-height: 100dvh; place-items: center; padding: 28px; text-align: center; }
    .status strong { display: block; margin-bottom: 8px; color: var(--sd-blue); font-size: 18px; }
    .status span { color: var(--sd-muted); font-size: 11px; }
    .status .incompatible { color: var(--sd-red); }
    @media (max-width: 900px), (max-height: 759px) {
      .shell { grid-template-rows: none; height: auto; min-height: 100dvh; overflow: visible; }
      .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .detail-grid, .history-grid { grid-template-columns: 1fr; }
      .summary-grid > * { min-height: 94px; }
      operational-efficiency-card, actor-lifecycle-card { min-height: 120px; }
      .detail-grid > *, .history-grid > * { min-height: 240px; }
    }
    @media (max-width: 520px) {
      .shell { padding: 9px; }
      header { align-items: flex-start; gap: 12px; }
      h1 span { display: block; margin: 4px 0 0; }
      .scope { display: none; }
      .summary-grid { grid-template-columns: 1fr; }
    }
  `;

  fixture = "";
  pollInterval = 2_000;
  snapshot?: DashboardSnapshot;
  message = "Loading dashboard…";
  frontendStale = false;
  incompatibleVersion = "";
  selectedActorNames: string[] = [];
  operationMessage = "";
  operationPending = false;

  private readonly api = new DashboardApiClient();
  private controller?: AbortController;
  private pollTimer?: number;
  private polling = false;

  connectedCallback(): void {
    super.connectedCallback();
    if (!this.fixture) this.fixture = new URLSearchParams(window.location.search).get("fixture") ?? "";
    void this.start();
  }

  disconnectedCallback(): void {
    this.stop();
    super.disconnectedCallback();
  }

  private async start(): Promise<void> {
    this.stop();
    if (this.fixture) {
      await this.loadFixture();
      return;
    }
    this.controller = new AbortController();
    try {
      const meta = await this.api.readMeta(this.controller.signal);
      if (meta.apiVersion !== supportedApiVersion) {
        this.incompatibleVersion = meta.apiVersion;
        return;
      }
      await this.refresh();
    } catch (error) {
      this.handleError(error);
      this.scheduleRefresh();
    }
  }

  private stop(): void {
    this.controller?.abort();
    this.controller = undefined;
    if (this.pollTimer !== undefined) window.clearTimeout(this.pollTimer);
    this.pollTimer = undefined;
  }

  private async loadFixture(): Promise<void> {
    this.controller = new AbortController();
    try {
      const response = await fetch(`/snapshots/${encodeURIComponent(this.fixture)}.json`, { signal: this.controller.signal });
      if (!response.ok) throw new Error(`Fixture ${this.fixture} is unavailable`);
      this.acceptSnapshot(await response.json() as DashboardSnapshot);
    } catch (error) {
      this.handleError(error);
    }
  }

  private async refresh(): Promise<void> {
    if (this.polling || this.incompatibleVersion) return;
    this.polling = true;
    this.controller = new AbortController();
    try {
      this.acceptSnapshot(await this.api.readSnapshot(this.controller.signal));
      this.frontendStale = false;
      this.message = "";
    } catch (error) {
      this.handleError(error);
    } finally {
      this.polling = false;
      this.scheduleRefresh();
    }
  }

  private scheduleRefresh(): void {
    if (!this.isConnected || this.fixture || this.incompatibleVersion || this.pollTimer !== undefined) return;
    this.pollTimer = window.setTimeout(() => {
      this.pollTimer = undefined;
      void this.refresh();
    }, this.pollInterval);
  }

  private acceptSnapshot(snapshot: DashboardSnapshot): void {
    if (snapshot.apiVersion !== supportedApiVersion) {
      this.incompatibleVersion = snapshot.apiVersion;
      return;
    }
    this.snapshot = snapshot;
    const available = new Set(snapshot.actors.map(actor => actor.name));
    this.selectedActorNames = this.selectedActorNames.filter(name => available.has(name));
  }

  private handleError(error: unknown): void {
    if ((error as DOMException).name === "AbortError") return;
    this.frontendStale = Boolean(this.snapshot);
    this.message = error instanceof Error ? error.message : "Dashboard unavailable";
  }

  private async operationRequested(event: CustomEvent<ActorOperationRequestDetail>): Promise<void> {
    if (this.operationPending) return;
    if (this.fixture) {
      this.operationMessage = "Fixture mode · operations disabled";
      return;
    }
    this.operationPending = true;
    this.operationMessage = `${event.detail.operation} in progress…`;
    try {
      const response = await this.api.operate(event.detail.operation, event.detail.actorNames);
      const failed = response.operation.results.filter(result => result.outcome === "FAILED").length;
      this.operationMessage = failed ? `${failed} of ${response.operation.results.length} requests failed` : `${response.operation.results.length} requests succeeded`;
      this.selectedActorNames = [];
      if (this.snapshot) {
        this.snapshot = { ...this.snapshot, operations: [response.operation, ...this.snapshot.operations.filter(operation => operation.id !== response.operation.id)] };
      }
      await this.refresh();
    } catch (error) {
      this.operationMessage = error instanceof Error ? error.message : "Operation failed";
    } finally {
      this.operationPending = false;
    }
  }

  protected render() {
    if (this.incompatibleVersion) {
      return html`<div class="status" role="alert"><div><strong class="incompatible">Incompatible dashboard API</strong><span>Frontend supports ${supportedApiVersion}; backend reported ${this.incompatibleVersion}. No data was rendered.</span></div></div>`;
    }
    const snapshot = this.snapshot;
    if (!snapshot) {
      return html`<div class="status" role="status"><div><strong>Agent Substrate Dashboard</strong><span>${this.message}</span></div></div>`;
    }
    const activeStates = snapshot.actorSummary.states.filter(state => state.count > 0).map(state => state.state as ActorState);
    return html`
      <main class="shell">
        <header>
          <div><h1>Agent Substrate <span>Fleet Operations</span></h1><div class="scope">${snapshot.scope.atespace} / ${snapshot.scope.workerPool}</div></div>
          <div>${this.operationMessage ? html`<div class="operation-message" role="status">${this.operationMessage}</div>` : ""}<time class="clock" datetime=${snapshot.observedAt}>${formatClock(snapshot.observedAt)}</time></div>
        </header>
        <section class="summary-grid" aria-label="Fleet summary">
          <actor-status-card .summary=${snapshot.actorSummary}></actor-status-card>
          <control-api-card .connection=${snapshot.connection} .frontendStale=${this.frontendStale}></control-api-card>
          <transition-count-card .summary=${snapshot.transitionSummary}></transition-count-card>
          <worker-occupancy-card .summary=${snapshot.workerSummary}></worker-occupancy-card>
        </section>
        <operational-efficiency-card .efficiency=${snapshot.efficiency} .workers=${snapshot.workers}></operational-efficiency-card>
        <actor-lifecycle-card .activeStates=${activeStates}></actor-lifecycle-card>
        <section class="detail-grid" aria-label="Fleet detail">
          <worker-map-card .workers=${snapshot.workers} .truncation=${snapshot.truncation.workers}></worker-map-card>
          <actor-fleet-card .actors=${snapshot.actors} .selectedActorNames=${this.selectedActorNames} .capabilities=${snapshot.capabilities} .truncation=${snapshot.truncation.actors} .busy=${this.operationPending} @actor-selection-change=${(event: CustomEvent<ActorSelectionChangeDetail>) => { this.selectedActorNames = [...event.detail.actorNames]; }} @actor-operation-request=${(event: CustomEvent<ActorOperationRequestDetail>) => { void this.operationRequested(event); }}></actor-fleet-card>
        </section>
        <section class="history-grid" aria-label="Fleet history">
          <actor-timeline-card .transitions=${snapshot.transitions}></actor-timeline-card>
          <operation-stream-card .operations=${snapshot.operations}></operation-stream-card>
        </section>
      </main>
    `;
  }
}

customElements.define("substrate-dashboard", SubstrateDashboard);

declare global {
  interface HTMLElementTagNameMap { "substrate-dashboard": SubstrateDashboard; }
}
