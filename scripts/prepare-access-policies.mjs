#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse, stringify } from "yaml";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const argumentsByName = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  argumentsByName.set(process.argv[index], process.argv[index + 1]);
}

const atespace = argumentsByName.get("--atespace");
const requestedRole = argumentsByName.get("--role") ?? "viewer";
const member =
  argumentsByName.get("--member") ??
  "user:system:serviceaccount:agent-substrate-dashboard:agent-substrate-dashboard";

if (!atespace || !["viewer", "editor"].includes(requestedRole)) {
  console.error(
    "usage: node scripts/prepare-access-policies.mjs --atespace NAME --role viewer|editor",
  );
  process.exit(2);
}

const outputDirectory = path.join(repositoryRoot, ".cache", "dashboard-access");
mkdirSync(outputDirectory, { recursive: true });

const roleRank = { viewer: 1, editor: 2, owner: 3 };

function readPolicy(scopeArguments) {
  const result = spawnSync(
    "kubectl",
    ["ate", "get", "access-policy", ...scopeArguments, "-o", "yaml"],
    { encoding: "utf8" },
  );

  if (result.status === 0) {
    return { policy: parse(result.stdout), action: "update" };
  }

  const error = `${result.stderr ?? ""}\n${result.stdout ?? ""}`;
  if (/not.?found|code\s*=\s*NotFound|code\s*=\s*5/i.test(error)) {
    return { policy: { metadata: { name: "default" }, bindings: [] }, action: "create" };
  }

  throw new Error(`could not read AccessPolicy:\n${error.trim()}`);
}

function mergeMember(policy, role) {
  policy.bindings ??= [];
  const strongestExistingRole = policy.bindings
    .filter((binding) => binding.members?.includes(member))
    .map((binding) => binding.role)
    .sort((left, right) => (roleRank[right] ?? 0) - (roleRank[left] ?? 0))[0];

  if ((roleRank[strongestExistingRole] ?? 0) >= roleRank[role]) {
    return false;
  }

  for (const binding of policy.bindings) {
    binding.members = (binding.members ?? []).filter((candidate) => candidate !== member);
  }
  policy.bindings = policy.bindings.filter((binding) => binding.members.length > 0);

  let target = policy.bindings.find((binding) => binding.role === role);
  if (!target) {
    target = { role, members: [] };
    policy.bindings.push(target);
  }
  target.members.push(member);
  target.members.sort();
  return true;
}

function prepare({ name, scopeArguments, role }) {
  const { policy, action } = readPolicy(scopeArguments);
  const changed = mergeMember(policy, role);
  const outputPath = path.join(outputDirectory, `${name}.yaml`);
  writeFileSync(outputPath, stringify(policy, { lineWidth: 0 }));
  return { name, scopeArguments, role, action, changed, outputPath };
}

const prepared = [
  prepare({ name: "global", scopeArguments: ["--global"], role: "viewer" }),
  prepare({ name: "atespace", scopeArguments: ["-a", atespace], role: requestedRole }),
];

console.log(`Prepared policy candidates for ${member}.`);
console.log("Review both files before running the commands below:\n");
for (const candidate of prepared) {
  console.log(`  ${candidate.outputPath} (${candidate.changed ? `adds ${candidate.role}` : "already sufficient"})`);
}
console.log("\nAfter review, apply with:\n");
for (const candidate of prepared.filter((item) => item.changed)) {
  const scope = candidate.scopeArguments.map((argument) => `'${argument}'`).join(" ");
  console.log(
    `kubectl ate ${candidate.action} access-policy ${scope} -f '${candidate.outputPath}'`,
  );
}
if (prepared.every((candidate) => !candidate.changed)) {
  console.log("# No policy update is required.");
}
