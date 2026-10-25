import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import YAML from "yaml";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoDir = path.resolve(scriptDir, "..");
const apiDocument = YAML.parse(
  fs.readFileSync(path.join(repoDir, "api/openapi.yaml"), "utf8"),
);
const fixtureManifest = JSON.parse(
  fs.readFileSync(path.join(repoDir, "api/fixtures/manifest.json"), "utf8"),
);

const schemas = apiDocument.components?.schemas;
if (!schemas) throw new Error("api/openapi.yaml has no components.schemas");

for (const [schemaName, schema] of Object.entries(schemas)) {
  if (!schema.description) throw new Error(`${schemaName} has no description`);
  for (const [propertyName, property] of Object.entries(schema.properties ?? {})) {
    if (!property.description && !property.$ref) {
      throw new Error(`${schemaName}.${propertyName} has no description`);
    }
  }
}

function dereference(value, stack = []) {
  if (Array.isArray(value)) return value.map((item) => dereference(item, stack));
  if (!value || typeof value !== "object") return value;

  if (typeof value.$ref === "string") {
    const prefix = "#/components/schemas/";
    if (!value.$ref.startsWith(prefix)) {
      throw new Error(`unsupported schema reference: ${value.$ref}`);
    }
    const name = value.$ref.slice(prefix.length);
    if (stack.includes(name)) throw new Error(`recursive schema reference: ${[...stack, name].join(" -> ")}`);
    if (!schemas[name]) throw new Error(`unknown schema reference: ${name}`);
    return dereference(schemas[name], [...stack, name]);
  }

  return Object.fromEntries(
    Object.entries(value).map(([key, child]) => [key, dereference(child, stack)]),
  );
}

const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);
let failures = 0;

for (const fixture of fixtureManifest.fixtures) {
  const schema = schemas[fixture.schema];
  if (!schema) throw new Error(`fixture names unknown schema: ${fixture.schema}`);
  const validate = ajv.compile(dereference(schema, [fixture.schema]));
  const fixturePath = path.join(repoDir, "api/fixtures", fixture.path);
  const data = JSON.parse(fs.readFileSync(fixturePath, "utf8"));
  if (!validate(data)) {
    failures += 1;
    process.stderr.write(`${fixture.path} does not match ${fixture.schema}\n`);
    process.stderr.write(`${JSON.stringify(validate.errors, null, 2)}\n`);
  }
}

if (failures > 0) process.exit(1);
process.stdout.write(`validated ${fixtureManifest.fixtures.length} fixtures\n`);
