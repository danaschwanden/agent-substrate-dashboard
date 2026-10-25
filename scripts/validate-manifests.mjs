import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  chmodSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { parse, parseAllDocuments } from "yaml";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function assertNoPrivatePathsOrNames(directory) {
  const ignoredDirectories = new Set([".cache", ".git", "dist", "node_modules", "test-results"]);
  const macHomePattern = new RegExp(["/", "Users", "/", "[^/\\s]+"].join(""), "i");
  const windowsHomePattern = new RegExp(
    ["[A-Z]:", "\\\\", "Users", "\\\\", "[^\\\\\\s]+"].join(""),
    "i",
  );
  const removedUsernamePattern = new RegExp(["dana", "schwanden"].join(""), "i");
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory() && ignoredDirectories.has(entry.name)) {
      continue;
    }
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      assertNoPrivatePathsOrNames(entryPath);
      continue;
    }
    if (!entry.isFile()) {
      continue;
    }
    const contents = readFileSync(entryPath, "utf8");
    assert.doesNotMatch(contents, macHomePattern, `${entryPath} contains a local macOS home path`);
    assert.doesNotMatch(contents, windowsHomePattern, `${entryPath} contains a local Windows home path`);
    assert.doesNotMatch(contents, removedUsernamePattern, `${entryPath} contains a personal username`);
  }
}

assertNoPrivatePathsOrNames(repositoryRoot);

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
assert.equal(namespace.metadata.labels["ate.dev/api-access"], "true");
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
assert.equal(kindServices.length, 2);
for (const service of kindServices) {
  assert.equal(service.spec.type, "ClusterIP", `${service.metadata.name} must remain internal`);
}

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

const kindVersionCheck = path.join(repositoryRoot, "scripts/check-kind-version.sh");
assert.throws(
  () =>
    execFileSync(kindVersionCheck, ["v0.26.0"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    }),
  (error) => error.status === 1 && error.stderr.includes("v0.27.0 or newer"),
);
for (const supportedVersion of ["v0.27.0", "v0.33.0", "v1.0.0"]) {
  execFileSync(kindVersionCheck, [supportedVersion], { encoding: "utf8" });
}

const policyTestDirectory = mkdtempSync(path.join(tmpdir(), "dashboard-policy-test-"));
try {
  const fakeKubectl = path.join(policyTestDirectory, "kubectl");
  writeFileSync(
    fakeKubectl,
    `#!/bin/sh
case " $* " in
  *" --global "*) exit 0 ;;
  *) printf '%s\\n' 'metadata:' '  name: default' '  uid: existing-policy' '  version: 7' 'bindings:' '  - role: viewer' '    members:' '      - user:existing-viewer' ;;
esac
`,
  );
  chmodSync(fakeKubectl, 0o755);
  const policyPreparation = execFileSync(
    process.execPath,
    [
      path.join(repositoryRoot, "scripts/prepare-access-policies.mjs"),
      "--atespace",
      "example",
      "--role",
      "viewer",
    ],
    {
      cwd: repositoryRoot,
      encoding: "utf8",
      env: {
        ...process.env,
        DASHBOARD_POLICY_OUTPUT_DIR: policyTestDirectory,
        PATH: `${policyTestDirectory}:${process.env.PATH}`,
      },
    },
  );
  assert.match(policyPreparation, /kubectl ate create access-policy '--global'/);
  assert.match(policyPreparation, /kubectl ate update access-policy '-a' 'example'/);
  const globalCandidate = parse(
    readFileSync(path.join(policyTestDirectory, "global.yaml"), "utf8"),
  );
  assert.deepEqual(globalCandidate.bindings, [
    {
      role: "viewer",
      members: [
        "user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard",
      ],
    },
  ]);
} finally {
  rmSync(policyTestDirectory, { recursive: true, force: true });
}

const publishWorkflow = parse(
  readFileSync(path.join(repositoryRoot, ".github/workflows/publish-images.yaml"), "utf8"),
);
assert.deepEqual(publishWorkflow.on.push.branches, ["main"]);
assert.deepEqual(publishWorkflow.on.push.tags, ["v*"]);
assert.equal(publishWorkflow.on.pull_request, undefined, "publishing must not run for pull requests");
const publishJob = publishWorkflow.jobs.publish;
assert.equal(publishJob.needs, "validate");
assert.equal(publishJob.permissions.contents, "read");
assert.equal(publishJob.permissions.packages, "write");
assert.deepEqual(publishJob.strategy.matrix.component, ["backend", "frontend"]);
const loginStep = publishJob.steps.find((step) => step.uses?.startsWith("docker/login-action@"));
assert.equal(loginStep?.with.registry, "ghcr.io");
assert.equal(loginStep?.with.password, "${{ secrets.GITHUB_TOKEN }}");
const buildStep = publishJob.steps.find((step) =>
  step.uses?.startsWith("docker/build-push-action@"),
);
assert.equal(buildStep?.with.platforms, "linux/amd64,linux/arm64");
assert.equal(buildStep?.with.push, true);
assert.match(buildStep?.with.tags ?? "", /steps\.meta\.outputs\.tags/);

console.log("Deployment manifests and container packaging invariants are valid.");
