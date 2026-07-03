// Fast, deterministic contract pre-flight (runs BEFORE the heavy oapi/redoc codegen).
// Resolves every local `$ref` in backend/api/crm.yaml and fails with a precise
// "path -> missing component" message. Catches the dangling-ref class of bug
// (e.g. a typo'd schema name like `ProblemDetail` when the schema is `Problem`)
// that otherwise only surfaces as a cryptic redoc "Can't resolve $ref" abort.
import { readFileSync } from "node:fs";
import { parse } from "yaml";

const path = process.argv[2] ?? "backend/api/crm.yaml";
const doc = parse(readFileSync(path, "utf8"));

// Walk the doc, collecting every {$ref: "..."} with a JSON-pointer trail to it.
const refs = [];
const visit = (node, trail) => {
  if (Array.isArray(node)) {
    node.forEach((v, i) => visit(v, `${trail}/${i}`));
  } else if (node && typeof node === "object") {
    for (const [k, v] of Object.entries(node)) {
      if (k === "$ref" && typeof v === "string") refs.push({ ref: v, at: trail });
      else visit(v, `${trail}/${k.replace(/~/g, "~0").replace(/\//g, "~1")}`);
    }
  }
};
visit(doc, "#");

// Resolve a local "#/a/b/c" pointer against the doc; null if any hop is missing.
const resolve = (ref) => {
  let cur = doc;
  for (const raw of ref.slice(2).split("/")) {
    const key = raw.replace(/~1/g, "/").replace(/~0/g, "~");
    if (cur && typeof cur === "object" && key in cur) cur = cur[key];
    else return null;
  }
  return cur;
};

const dangling = [];
for (const { ref, at } of refs) {
  if (!ref.startsWith("#/")) continue; // external refs are out of scope here
  if (resolve(ref) === null) dangling.push({ ref, at });
}

if (dangling.length > 0) {
  console.error(`contract-lint: ${dangling.length} dangling $ref(s) in ${path}:`);
  for (const { ref, at } of dangling) console.error(`  ${at}\n    -> ${ref} (no such component)`);
  console.error("Fix the ref target (check for a typo'd schema name) and re-run.");
  process.exit(1);
}
console.log(`contract-lint: ${refs.length} $ref(s) resolve cleanly in ${path}`);
