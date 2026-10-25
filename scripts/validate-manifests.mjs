import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { parseAllDocuments } from "yaml";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function render(relativePath) {
  const rendered = execFileSync("kubectl", ["kustomize", relativePath], {
    cwd: repositoryRoot,
    encoding: "utf8",
  });

  return parseAllDocuments(rendered)
    .map((document) => {
      if (document.errors.length > 0) {
        throw document.errors[0];
      }
      return document.toJS();
    })
    .filter(Boolean);
}

function resourcesOfKind(resources, kind) {
  return resources.filter((resource) => resource.kind === kind);
}

function named(resources, kind, name) {
  const resource = resources.find(
    (candidate) => candidate.kind === kind && candidate.metadata?.name === name,
  );
  assert(resource, `${kind}/${name} must be present`);
  return resource;
}

function assertHardenedDeployment(deployment) {
  const pod = deployment.spec.template.spec;
  assert.equal(pod.automountServiceAccountToken, false);
  assert.equal(pod.securityContext.runAsNonRoot, true);
  assert.equal(pod.securityContext.seccompProfile?.type, "RuntimeDefault");

  assert.equal(pod.containers.length, 1);
  const [container] = pod.containers;
  assert.equal(container.securityContext.allowPrivilegeEscalation, false);
  assert.equal(container.securityContext.readOnlyRootFilesystem, true);
  assert(container.securityContext.capabilities?.drop?.includes("ALL"));
  assert(container.startupProbe, `${deployment.metadata.name} needs a startup probe`);
  assert(container.livenessProbe, `${deployment.metadata.name} needs a liveness probe`);
  assert(container.readinessProbe, `${deployment.metadata.name} needs a readiness probe`);
  assert(container.resources?.requests, `${deployment.metadata.name} needs resource requests`);
  assert(container.resources?.limits, `${deployment.metadata.name} needs resource limits`);
}

function assertNoAccessPolicy(resources, source) {
  assert.equal(
    resourcesOfKind(resources, "AccessPolicy").length,
    0,
    `${source} must not manage Agent Substrate AccessPolicy`,
  );
}

const base = render("deploy/kustomize/base");
const kind = render("deploy/kustomize/overlays/kind");
const digestPinned = render("deploy/kustomize/examples/digest-pinned");

for (const [source, resources] of [
  ["base", base],
  ["kind overlay", kind],
  ["digest example", digestPinned],
]) {
  assertNoAccessPolicy(resources, source);
  assert.equal(resourcesOfKind(resources, "Deployment").length, 2);
}

const baseServices = resourcesOfKind(base, "Service");
assert.equal(baseServices.length, 2);
for (const service of baseServices) {
  assert.equal(service.spec.type, "ClusterIP", `${service.metadata.name} must be internal by default`);
}

const baseDeployments = resourcesOfKind(base, "Deployment");
for (const deployment of baseDeployments) {
  assertHardenedDeployment(deployment);
}

const namespace = named(base, "Namespace", "agent-substrate-dashboard");
for (const mode of ["enforce", "audit", "warn"]) {
  assert.equal(namespace.metadata.labels[`pod-security.kubernetes.io/${mode}`], "restricted");
}

const config = resourcesOfKind(base, "ConfigMap").find((resource) =>
  resource.metadata.name.startsWith("agent-substrate-dashboard-config-"),
);
assert(config, "generated dashboard ConfigMap must be present");
assert.equal(config.data.DASHBOARD_ENABLE_OPERATIONS, "false");

const backend = named(base, "Deployment", "agent-substrate-dashboard-backend");
assert.equal(backend.spec.template.spec.serviceAccountName, "agent-substrate-dashboard");
const identity = backend.spec.template.spec.volumes.find((volume) => volume.name === "identity");
assert(identity?.projected, "backend identity must be a projected volume");
const projectedSources = identity.projected.sources;
const token = projectedSources.find((source) => source.serviceAccountToken)?.serviceAccountToken;
assert.equal(token?.audience, "api.ate-system.svc");
assert.equal(token?.path, "token");
const trustBundle = projectedSources.find((source) => source.clusterTrustBundle)?.clusterTrustBundle;
assert.equal(trustBundle?.signerName, "servicedns.podcert.ate.dev/identity");
assert.equal(trustBundle?.path, "trust-bundle.pem");

const policy = named(base, "NetworkPolicy", "agent-substrate-dashboard-backend");
assert.deepEqual(policy.spec.policyTypes, ["Ingress"]);
const allowedComponents = policy.spec.ingress.flatMap((rule) => rule.from ?? []).map(
  (peer) => peer.podSelector?.matchLabels?.["app.kubernetes.io/component"],
);
assert.deepEqual(allowedComponents, ["frontend"]);

const kindServices = resourcesOfKind(kind, "Service");
const exposedServices = kindServices.filter((service) => service.spec.type !== "ClusterIP");
assert.equal(exposedServices.length, 1, "kind must expose exactly one service");
assert.equal(exposedServices[0].metadata.name, "agent-substrate-dashboard-frontend");
assert.equal(exposedServices[0].spec.type, "NodePort");
assert.equal(exposedServices[0].spec.ports[0].nodePort, 30090);
assert.equal(
  named(kind, "Service", "agent-substrate-dashboard-backend").spec.type,
  "ClusterIP",
);

for (const deployment of resourcesOfKind(digestPinned, "Deployment")) {
  const image = deployment.spec.template.spec.containers[0].image;
  assert.match(image, /@sha256:[a-f0-9]{64}$/);
}

for (const relativePath of ["backend/Dockerfile", "frontend/Dockerfile"]) {
  const dockerfile = readFileSync(path.join(repositoryRoot, relativePath), "utf8");
  assert.match(dockerfile, /^FROM .+ AS build$/m, `${relativePath} must use a build stage`);
  assert.equal(
    dockerfile.match(/^FROM .+$/gm)?.length,
    2,
    `${relativePath} must define separate build and runtime stages`,
  );
}
const backendDockerfile = readFileSync(path.join(repositoryRoot, "backend/Dockerfile"), "utf8");
assert.match(backendDockerfile, /ARG TARGETARCH/);
assert.match(backendDockerfile, /GOARCH=\$\{TARGETARCH\}/);

console.log("Deployment manifests and container packaging invariants are valid.");
